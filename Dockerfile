FROM balenalib/beaglebone-black-node:6 as frontend

RUN apt update && apt install -y jq
RUN npm install --global yarn ember-cli

# Add source and copy checkout frontend submodule
WORKDIR /usr/src/app/
ADD frontend.sh /usr/src/
RUN /usr/src/frontend.sh

# Install depenandcies
RUN yarn
RUN ember build

# build main runtime image
FROM balenalib/beaglebone-black-golang:1.13

# Enable systemd
ENV INITSYSTEM on
ENV SRCDIR $GOPATH/src/github.com/CodedInternet/godynastat
ENV HTMLDIR $GOPATH/html

# get the godep tool
RUN go get -u github.com/golang/dep/cmd/dep

# install minicom for debugging purposes
RUN apt update && apt install minicom

# Add souce files
COPY . $SRCDIR

# Run our custom build script
RUN bash $SRCDIR/build.sh
COPY bbb_config.yaml run.sh $GOPATH/

# Add frontend
COPY --from=frontend /usr/src/app/dist/ $HTMLDIR

CMD ["bash", "run.sh"]
