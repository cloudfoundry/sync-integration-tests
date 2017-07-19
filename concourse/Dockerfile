FROM ubuntu:trusty

RUN \
  echo "****** This is my working directory ******" && \
  pwd && \
  echo && \
  echo "++++++ Here's what's in my root directory ++++++++" && \
  ls -hla --color=tty && \
  echo && \
  echo "====== Here's what's one step above me ======" && \
  ls -hla .. && \
  echo

RUN \
  apt-get update && \
  apt-get -y install \
    build-essential \
    wget \
    curl \
    openssh-client \
    software-properties-common \
    unzip

RUN \
  add-apt-repository ppa:git-core/ppa -y && \
  apt-get update && \
  apt-get install -y git && \
  apt-get remove -y --purge software-properties-common

ENV GOPATH /go
ENV PATH /go/bin:/usr/local/go/bin:$PATH
RUN \
  wget https://storage.googleapis.com/golang/go1.7.4.linux-amd64.tar.gz -P /tmp && \
  tar xzvf /tmp/go1.7.4.linux-amd64.tar.gz -C /usr/local && \
  mkdir $GOPATH && \
  rm -rf /tmp/*

RUN go get github.com/onsi/ginkgo/ginkgo

# Install jq
RUN wget -O /usr/local/bin/jq 'https://github.com/stedolan/jq/releases/download/jq-1.5/jq-linux64' && \
  chmod +x /usr/local/bin/jq

# Install bbl
RUN wget "$(curl -s https://api.github.com/repos/cloudfoundry/bosh-bootloader/releases | jq -r 'map(select((.prerelease == false) and (.draft == false) and (.tag_name|startswith("v3"))))[0].assets[].browser_download_url' | grep _linux)" && \
  mv bbl-* /usr/local/bin/bbl && \
  chmod +x /usr/local/bin/bbl

# Install the golang bosh CLI
RUN \
  wget https://s3.amazonaws.com/bosh-cli-artifacts/bosh-cli-0.0.147-linux-amd64 --output-document="/usr/bin/bosh" && \
  cd /usr/bin && \
  echo "533342d7663c3e5dc731769e157608c74dd9eccb bosh" | sha1sum -c - && \
  chmod +x bosh

# Install the cf CLI
RUN \
  wget -q -O cf.deb "https://cli.run.pivotal.io/stable?release=debian64&version=6.23.1&source=github-rel" && \
  dpkg -i cf.deb
