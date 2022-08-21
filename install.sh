#!/bin/bash

printf "Installing go...\n"
curl -OL https://go.dev/dl/go1.19.linux-amd64.tar.gz

sudo tar -C /usr/local -xvf go1.19.linux-amd64.tar.gz

printf "export PATH=\$PATH:/usr/local/go/bin" >> ~/.profile

source ~/.profile

mkdir -p ~/go/src/github.com/glxiia

cd ~/go/src/github.com/glxiia

printf "Cloning repo to go working directory...\n"
git clone https://github.com/glxiia/opswat_engr.git

cd opswat_engr
