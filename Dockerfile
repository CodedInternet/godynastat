FROM resin/beaglebone-black-golang:1.8

# Enable systemd
ENV INITSYSTEM on
ENV SRCDIR $GOPATH/src/github.com/CodedInternet/godynastat

# get the godep tool
RUN go get -u github.com/golang/dep/cmd/dep

# Add souce files
COPY . $SRCDIR

# Run our custom build script
RUN bash $SRCDIR/build.sh

CMD ["bash", "run.sh"]
