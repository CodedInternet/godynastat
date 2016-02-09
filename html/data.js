var pc_config = {"iceServers": [{"url": "stun:stun.stunprotocol.prg"}]};
var wsuri = "ws://127.0.0.1:8000/ws/device/test/";

var pc = new webkitRTCPeerConnection(pc_config);

var dcOptions = {
    ordered: true,
    reliable: false
}

pc.ondatachannel = function(event) {
    receiveChannel = event.channel;
    receiveChannel.onmessage = function(event){
        console.log(event.data);
    };
};

pc.onicecandidate = function(event) {
    if(pc.iceGatheringState == "complete") {
        //pc.setLocalDescription(pc.localDescription)
        //console.log(JSON.stringify(pc.localDescription))
    }
}

pc.icegatheringstatechange = function(event) {
    if(pc.iceGatheringState == "complete") {
        console.log(JSON.stringify(pc.localDescription))
    }
}

var dc = pc.createDataChannel('data', dcOptions);

dc.onmessage = function(event) {
    console.log("Incomming message: " + event.data);
}

dc.onopen = function () {
    dc.send(JSON.stringify({message: "Hello World!"}));
};

dc.onclose = function () {
    console.log("The Data Channel is Closed");
};

function getOffer() {
    pc.createOffer(function(desc) {
        pc.setLocalDescription(desc);
        doSend(JSON.stringify(desc));
    })
}

function getAnswer(offer) {
    pc.setRemoteDescription(new RTCSessionDescription(offer));
    pc.createAnswer(function(desc) {
        pc.setLocalDescription(desc);
        console.log(desc);
    })
}

function gotSignal(signal) {
    if (signal.type == "answer")
        pc.setRemoteDescription(new RTCSessionDescription(signal), function(event) {
            console.log("Answer set");
        });
    else
        pc.addIceCandidate(new RTCIceCandidate(signal));
}

function openRtc() {
    websocket = new WebSocket(wsuri);
    websocket.onopen = function(evt) {getOffer()};
    websocket.onmessage = function(evt) {
        console.log(evt.data);
        gotSignal(JSON.parse(evt.data));
    };
}

function doSend(message) {
    websocket.send(message);
}