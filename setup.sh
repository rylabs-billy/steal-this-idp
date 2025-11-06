#!/bin/bash

# Generate and configure secrets for Pulumi ESC, Linode Object Storage, and Age.
# Requires a Linode API token and Pulumi Access Token
#
# Author: Billy Thompson

set -e
trap "cleanup $?" EXIT SIGINT

PLATFORM_LABEL=$1

readonly GREEN="\e[1;32m"
readonly RED="\e[1;31m"
readonly GREY="\e[1;37m"
readonly MAGENTA="\e[1;35m"
readonly RESET="\e[m"

readonly basedir=$(pwd)
readonly bindir="$HOME/.local/bin"
readonly pad=$(printf "%-6s" "")

declare -A regions
# APJ
regions["1"]="ap-south"
regions["2"]="au-mel"
regions["3"]="id-cgk"
regions["4"]="jp-osa"
# EU
regions["5"]="es-mad"
regions["6"]="fr-par"
regions["7"]="gb-lon"
regions["8"]="it-mil"
regions["9"]="nl-ams"
regions["10"]="se-sto"
# Americas
regions["11"]="br-gru"
regions["12"]="us-east"
regions["13"]="us-lax"
regions["14"]="us-mia"
regions["15"]="us-ord"
regions["16"]="us-sea"
regions["17"]="us-southeast"


cleanup() {
  if [ "$?" != "0" ]; then
    tput cnorm
    exit 1
  fi
}

usage() {
  printf "
${MAGENTA}usage:${GREY} ./setup.sh [PLATFORM_LABEL]${RESET}

${MAGENTA}[info]${GREEN} set environment variables before running script again${RESET}
       ${GREY}export LINODE_TOKEN=<TOKEN>${RESET}
       ${GREY}export PULUMI_ACCESS_TOKEN=<TOKEN>${RESET}

       ${GREEN}optionally, update the editor if using vscode${RESET}
       ${GREY}export EDITOR=code${RESET}

       ${GREEN}update shell profile to persist for subsequent pulumi runs${RESET}
       ${GREY}echo \"# APL Demo\" >> ~/.bashrc${RESET}
       ${GREY}echo \"LINODE_TOKEN=\$LINODE_TOKEN\" >> ~/.bashrc${RESET}
       ${GREY}echo \"PULUMI_ACCESS_TOKEN=\$PULUMI_ACCESS_TOKEN\" >> ~/.bashrc${RESET}\n"
  exit 0
}


err() {
  case $1 in
    install) printf "\n${RED}[error]${GREY} $2 installation is required${RESET}";;
       root) printf "\n${RED}[error]${GREY} do not run as root user${RESET}\n";;
     region) printf "\n${RED}[error]${GREY} $2 region selection is invalid${RESET}\n";;
          *) usage;;
  esac
  [ "$2" == "homebrew" ] && printf "\n${GREY}[info]${GREY} https://docs.brew.sh/Installation${RESET}"
  return 1
}

msg() {
  case $1 in
         age) printf "\n${GREEN}[info]${GREY} generating age keys${RESET}\n";;
         api) printf "\n${GREEN}[info]${GREY} checking for pulumi and linode api tokens${RESET}\n";;
        brew) printf "\n${GREEN}[info]${GREY} checking for homebrew${RESET}\n";;
    complete) printf "\n${GREEN}[info]${GREY} setup complete${RESET}\n";;
        edit) printf "\n${GREEN}[info]${GREY} opening esc environment $2${RESET}\n";;
         env) printf "\n${GREEN}[info]${GREY} creating esc environment $2${RESET}\n";;
       input) printf "\n${GREEN}[info]${GREY} getting user inputs${RESET}\n";;
     install) printf "\n${GREEN}[info]${GREY} $2 is not installed${RESET}\n";;
       login) printf "\n${GREEN}[info]${GREY} logging into pulumi${RESET}\n";;
        pass) printf "\n${GREEN}[info]${GREY} generating password for $2${RESET}\n";;
        path) printf "\n${GREEN}[info]${GREY} update $2 to add aplcli to your system path${RESET}\n";;
      prompt) printf "\n${GREEN}[info]${GREY} install it now (y/n)${RESET}";;
      pulumi) printf "\n${GREEN}[info]${GREY} configuring pulumi${RESET}\n";;
        root) printf "\n${GREEN}[info]${GREY} checking for root${RESET}\n";;
         set) printf "\n${GREEN}[info]${GREY} setting esc value $2${RESET}\n";;
       stack) printf "\n${GREEN}[info]${GREY} initializing pulumi stack $2${RESET}\n";;
  esac
}

pre_chk() {
  # check env
  msg api
  [[ -z $LINODE_TOKEN ]] || [[ -z $PULUMI_ACCESS_TOKEN ]] && err

  # ensure not root
  msg root
  [ $(id -u) != "0" ] && return
  err root

  # check for homebrew installation
  msg brew
  homebrew=$(which brew)
  [ -n "$homebrew" ] && return
  err install homebrew
}

print_regions() {
  echo -e "\n${GREY}$pad $1${RESET}"
  for i in $(seq $2 $3); do
    echo -e "$pad ($i)    ${regions[$i]}"
  done
}

get_inputs() {
  # get user provided inputs
  msg input
  echo -ne "\n${GREEN}$pad Domain: ${RESET}"
  read -r DOMAIN

  echo -ne "\n${GREEN}$pad Email: ${RESET}"
  read -r EMAIL

  print_regions "APJ" 1 4
  print_regions "Europe" 5 10
  print_regions "Americas" 11 17
  echo -ne "\n${GREEN}$pad Linode Region: ${RESET}"
  read -r REGION

  [[ ! regions[$REGION] ]] && err region $REGION || echo yay
}

random_pass() {
  # generate random password that meets keycloak requirements
  p=$(uuidgen | md5sum | base64)
  pass=$(echo $p | fold -w1 | shuf | tr -d '\n')
  echo -ne $pass
}

pulumi_setup() {
  # configure pulumi esc secrets
  msg pulumi
  local aplstack="apl-demo/dev"
  local infrastack="apl-demo-infra/dev"

  if [ ! -d "$HOME/.pulumi/bin" ]; then
    msg install pulumi
    read -p "$(msg prompt) " ANSWER
    if [ "$ANSWER" == "Y" ] || [ "$ANSWER" == "y" ]; then
      curl -fsSL https://get.pulumi.com | sh
    else
      err install pulumi
    fi
  fi

  # login and init the infra stack
  cd $basedir/cmd/infra
  msg login && pulumi login
  local user=$(pulumi whoami)
  msg stack "$user/$infrastack"
  pulumi stack init $user/$infrastack

  # init the apl stack
  cd $basedir/cmd/apl
  msg stack "$user/$aplstack"
  pulumi stack init $user/$aplstack

  # configure the shared esc environment
  msg env $aplstack && esc env init $aplstack
  msg set linode.token && esc env set $aplstack linode.token $LINODE_TOKEN --secret

  [[ -z $PLATFORM_LABEL ]] && PLATFORM_LABEL="apl-demo"
  local region="${regions[$REGION]}"
  msg set apl.inputs.label && esc env set $aplstack apl.inputs.label $PLATFORM_LABEL
  msg set apl.inputs.domain && esc env set $aplstack apl.inputs.domain $DOMAIN
  msg set apl.inputs.email && esc env set $aplstack apl.inputs.email $EMAIL
  msg set apl.inputs.region && esc env set $aplstack apl.inputs.region $region

  msg set apl.slug.infra && esc env set $aplstack apl.slug.infra $user/$infrastack
  msg set apl.slug.apl && esc env set $aplstack apl.slug.apl $user/$aplstack

  msg set apl.age.publicKey && esc env set $aplstack apl.age.publicKey $age_public_key
  msg set apl.age.privateKey && esc env set $aplstack apl.age.privateKey $age_secret_key --secret

  for i in develop loki otomi; do
    _pass=$(random_pass)
    case $i in
      develop) local target="team.develop.password";;
      loki) local target="loki.adminPassword";;
      otomi) local target="otomi.adminPassword";;
    esac
    msg set apl.$target
    esc env set $aplstack apl.$target $_pass --secret
  done

  # manually update variable references in esc environment
  msg edit $aplstack
  pulumi env edit $aplstack
}

age_setup() {
  # generate age provider sops keys
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
  age_public_key=$(echo -ne $keypair | awk -F ' ' '{print $7}' | tr -d '\n')
  age_secret_key=$(echo -ne $keypair | awk -F ' ' '{print $8}' | tr -d '\n')
}

build_apl() {
  cd $basedir/cmd/automation
  go build -o aplcli
  mkdir -p $bindir
  cd $bindir
  ln -sf $basedir/cmd/automation/aplcli aplcli
  # find bash conf
  if [[ ! -f $HOME/.bashrc ]]; then
    local bashconf=$HOME/.bash_profile
  else
    local bashconf=$HOME/.bashrc
  fi

  # prompt to set path
  local path=$(echo $PATH | grep -o $bindir)
  if [[ -z $path ]]; then
    msg path $bashconf
    printf "\n\n\t${GREY}echo >> $bashconf${RESET}"
    printf "\n\t${GREY}echo "# app platform" >> $bashconf${RESET}"
    printf "\n\t${GREY}echo "export PATH=\$PATH:$bindir" >> $bashconf${RESET}\n"
  fi
}

# main
pre_chk
get_inputs
age_setup
pulumi_setup
build_apl
msg complete
