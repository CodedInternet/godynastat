var pc_config = {"iceServers": [{"url": "stun:stun.stunprotocol.prg"}]};
var wsuri = "ws://127.0.0.1:8000/ws/device/test/";

var pc = new webkitRTCPeerConnection(pc_config);

var dcOptions = {
    ordered: true,
    reliable: false
}

var sensor_map = [];

pc.ondatachannel = function(event) {
    console.log(event);
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
    var json = JSON.parse(event.data);
    for(var s_name in json["sensors"]) {
        var sensor = json["sensors"][s_name];
        for (var row in sensor) {
            for (var col in sensor[row]) {
                var id = s_name + "_" + row + "_" + col;
                if(!sensor_map.hasOwnProperty(id)) {
                    sensor_map[id] = $("#" + id);
                }
                sensor_map[id].text(sensor[row][col]);
            }
        }
    }
}

dc.onclose = function () {
    console.log("The Data Channel is Closed");
};

function getOffer(callback) {
    pc.createOffer(function(desc) {
        pc.setLocalDescription(desc);
        callback(JSON.stringify(pc.localDescription));
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
    if (signal.type == "answer") {
        pc.setRemoteDescription(new RTCSessionDescription(signal), function (event) {
            console.log("Answer set");
        });
    } else if (signal.type = "ice") { // TODO[ghost]: Lookup exectly what the correct syntax for this is.
        pc.addIceCandidate(new RTCIceCandidate(signal));
    }
}

function openRtc() {
    websocket = new WebSocket(wsuri);
    websocket.onopen = function (evt) {
        getOffer(doSend)
    };
    websocket.onmessage = function(evt) {
        jmessage = JSON.parse(evt.data)
        console.log(jmessage);
        if (jmessage.type = "answer") {
            gotSignal(jmessage);
        }
    };
}

function doSend(message) {
    websocket.send(message);
}
