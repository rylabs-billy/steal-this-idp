#!/bin/bash

# Generate and configure secrets for Pulumi ESC, Linode Object Storage, and Age.
# Requires a Linode API token and Pulumi Access Token
#
# Author: Billy Thompson

set -e
trap "cleanup $?" EXIT SIGINT

GREEN="\e[1;32m"
RED="\e[1;31m"
GREY="\e[1;37m"
MAGENTA="\e[1;35m"
RESET="\e[m"

cleanup() {
  if [ "$?" != "0" ]; then
    tput cnorm
    exit 1
  fi
}

usage() {
  printf "
${GREEN}Usage:${GREY} . ./setup.sh${RESET}

Note the dot (.) before the script file. The sourcing is optional, but it's
there to make your life easier by setting ${GREY}\$LINODE_TOKEN${RESET} and
${GREY}\$PULUMI_ACCESS_TOKEN${RESET} environment variables in the parent shell.
Copy and paste the below commands to persist them for subsequent Pulumi runs.

${GREEN}echo${GREY} \"# APL Demo\" >> ~/.bashrc
${GREEN}echo${GREY} \"LINODE_TOKEN=${MAGENTA}\$LINODE_TOKEN${GREY}\" >> ~/.bashrc
${GREEN}echo${GREY} \"PULUMI_ACCESS_TOKEN=${MAGENTA}\$PULUMI_ACCESS_TOKEN${GREY}\" >> ~/.bashrc
${RESET}\n"
}

err() {
  case $1 in
    install) printf "\n${RED}[error]${GREY} $2 installation is required${RESET}";;
       root) printf "\n${RED}[error]${GREY} do not run as root user${RESET}\n";;
  esac
  [ "$2" == "homebrew" ] && printf "\n${GREY}[info]${GREY} https://docs.brew.sh/Installation${RESET}"
  return 1
}

msg() {
  case $1 in
         age) printf "\n${GREEN}[info]${GREY} generating age keys${RESET}\n";;
        brew) printf "\n${GREEN}[info]${GREY} checking for homebrew${RESET}\n";;
    complete) printf "\n${GREEN}[info]${GREY} setup complete${RESET}\n";;
        edit) printf "\n${GREEN}[info]${GREY} opening esc environment $2${RESET}\n";;
         env) printf "\n${GREEN}[info]${GREY} creating esc environment $2${RESET}\n";;
     install) printf "\n${GREEN}[info]${GREY} $2 is not installed${RESET}\n";;
      linode) printf "\n${GREEN}[info]${GREY} setting up linode environment${RESET}\n";;
       login) printf "\n${GREEN}[info]${GREY} logging into pulumi${RESET}\n";;
         obj) printf "\n${GREEN}[info]${GREY} creating object storage keys${RESET}\n";;
        pass) printf "\n${GREEN}[info]${GREY} generating password for $2${RESET}\n";;
      prompt) printf "\n${GREEN}[info]${GREY} install it now (y/n)${RESET}";;
      pulumi) printf "\n${GREEN}[info]${GREY} configuring pulumi${RESET}\n";;
        root) printf "\n${GREEN}[info]${GREY} checking for root${RESET}\n";;
         set) printf "\n${GREEN}[info]${GREY} setting esc value $2${RESET}\n";;
       stack) printf "\n${GREEN}[info]${GREY} initializing pulumi stack $2${RESET}\n";;
  esac
}

root_chk() {
  # Ensure user is not root
  msg root
  [ $(id -u) != "0" ] && return
  err root
}

get_token() {
  # Get Pulumi and Linode API Tokens
  tput civis
  case $1 in
    linode)
      local prompt="Linode API Token:"
      echo -ne "\n${GREEN}Linode API Token: \033[0K\r${RESET}"
      read -rs token
      export LINODE_TOKEN="$token"
      ;;
    pulumi)
      local prompt="Pulumi Access Token:"
      echo -ne "\n${GREEN}$prompt \033[0K\r${RESET}"
      read -rs token
      export PULUMI_ACCESS_TOKEN="$token"
      ;;
  esac

  char_count=$(echo $token | tr -d '\n' | wc -c)
  for i in $(seq $char_count); do
    local char+='*'
  done

  echo -ne "${GREEN}$prompt ${GREY}$char \033[0K\r${RESET}\n"
  tput cnorm
}

brew_chk() {
  # Check if homebrew is installed
  msg brew
  homebrew=$(which brew)
  [ -n "$homebrew" ] && return
  err install homebrew
}

random_pass() {
  # Generate random password that meets Keycloak requirements
  p=$(uuidgen | md5sum | base64)
  pass=$(echo $p | fold -w1 | shuf | tr -d '\n')
  echo -e $pass
}

linode_setup() {
  msg linode
  # Setup Linode environment
  [ -z $LINODE_TOKEN ] && get_token linode
  
  msg obj
  obj=($(curl -s --request POST \
    --url https://api.linode.com/v4/object-storage/keys \
    --header 'accept: application/json' \
    --header "authorization: Bearer $LINODE_TOKEN" \
    --header 'content-type: application/json' \
    --data '{ "label": "apl-demo-key" }' | jq -r .label,.access_key,.secret_key))
}

pulumi_setup() {
  # Configure Pulumi ESC secrets
  msg pulumi
  local stack="apl-demo/dev"

  if [ ! -d "$HOME/.pulumi/bin" ]; then
    msg install pulumi
    read -p "$(msg prompt) " ANSWER
    if [ "$ANSWER" == "Y" ] || [ "$ANSWER" == "y" ]; then
      curl -fsSL https://get.pulumi.com | sh
    else
      err install pulumi
    fi
  fi

  [ -z $PULUMI_ACCESS_TOKEN] && get_token pulumi

  cd ./apl
  msg login && pulumi login
  msg env $stack && esc env init $stack
  msg set linode.token && esc env set $stack linode.token $LINODE_TOKEN --secret
  msg set linode.objAccessKey && esc env set $stack linode.objAccessKey "${obj[1]}"
  msg set linode.objSecretKey && esc env set $stack linode.objSecretKey "${obj[2]}" --secret
  msg set apl.age.publicKey && esc env set $stack apl.age.publicKey $age_public_key
  msg set apl.age.privateKey && esc env set $stack apl.age.privateKey $age_secret_key --secret

  for i in develop loki otomi; do
    _pass=$(random_pass)
    case $i in
      develop) local target="team.develop.password";;
      loki) local target="loki.adminPassword";;
      otomi) local target="otomi.adminPassword";;
    esac
    msg set apl.$target
    esc env set $stack apl.$target $_pass --secret
  done

  local user=$(pulumi whoami)
  msg stack "$user/$stack"
  pulumi stack init $user/$stack

  msg edit $stack
  pulumi env edit $stack
}

age_setup() {
  # Generate Age provider SOPS keys
  local age=$(which age)

  if [ -z "$age" ]; then
    msg install age
    read -p "$(msg prompt) " ANSWER
    if [ "$ANSWER" == "Y" ] || [ "$ANSWER" == "y" ]; then
      brew install age
    else
      err install age
    fi
  fi

  msg age
  keypair=$(age-keygen 2> /dev/null)
  age_public_key=$(echo $keypair | awk -F ' ' '{print $7}')
  age_secret_key=$(echo $keypair | awk -F ' ' '{print $8}')
}

# main
root_chk
brew_chk
age_setup
linode_setup
pulumi_setup
msg complete
