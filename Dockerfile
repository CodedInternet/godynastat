FROM resin/beaglebone-black-golang:1.8

# Enable systemd
ENV INITSYSTEM on
ENV SRCDIR $GOPATH/src/github.com/CodedInternet/godynastat
COPY . $SRCDIR

RUN bash $SRCDIR/onboard/build.sh

CMD ["bash", "run.sh"]
