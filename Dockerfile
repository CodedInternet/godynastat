FROM resin/beaglebone-black-node:6 as frontend

RUN npm install --global yarn ember-cli

# Add source and copy checkout frontend submodule
WORKDIR /usr/src/app/
# Create a directory for the cache buster to use and copy it
RUN mkdir /usr/src/goapp/
COPY . /usr/src/goapp/
RUN git clone https://github.com/CodedInternet/dynastat-frontend.git .

# Install depenandcies
RUN yarn
RUN ember build

# build main runtime image
FROM resin/beaglebone-black-golang:1.8

# Enable systemd
ENV INITSYSTEM on
ENV SRCDIR $GOPATH/src/github.com/CodedInternet/godynastat
ENV HTMLDIR $GOPATH/html

# get the godep tool
RUN go get -u github.com/golang/dep/cmd/dep

# Add souce files
COPY . $SRCDIR

# Run our custom build script
RUN bash $SRCDIR/build.sh
COPY bbb_config.yaml run.sh $GOPATH/

# Add frontend
COPY --from=frontend /usr/src/app/dist/ $HTMLDIR

CMD ["bash", "run.sh"]
