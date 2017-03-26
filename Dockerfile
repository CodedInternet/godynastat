FROM resin/beaglebone-golang:1.8

# Enable systemd
ENV INITSYSTEM on
ENV SRCDIR $GOPATH/src/github.com/CodedInternet/godynastat
COPY . $SRCDIR

RUN bash $SRCDIR/onboard/build.sh

CMD $SRCDIR/onboard/run.sh
