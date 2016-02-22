// Generated by CoffeeScript 1.10.0
(function() {
  (function($) {
    var Conductor, SignalingSocket, load_stun, stun_servers;
    stun_servers = [];
    SignalingSocket = (function() {
      function SignalingSocket(wsuri) {
        this.ws = new WebSocket(wsuri);
        this.ws.onclose = this.onclose;
        this.ws.onmessage = (function(_this) {
          return function(event) {
            return _this.conductor.onssmessage(event);
          };
        })(this);
      }

      SignalingSocket.prototype.setConductor = function(conductor1) {
        this.conductor = conductor1;
      };

      SignalingSocket.prototype.onclose = function(event) {
        if (event.code >= event.code.CLOSE_PROTOCOL_ERROR) {
          return alert("Could not connect to signaling server");
        }
      };

      SignalingSocket.prototype.send = function(msg) {
        return this.ws.send(msg);
      };

      return SignalingSocket;

    })();
    Conductor = (function() {
      function Conductor(signal_socket1) {
        var i, j, len, pc_config, url;
        this.signal_socket = signal_socket1;
        pc_config = {
          "iceServers": []
        };
        for (url = j = 0, len = stun_servers.length; j < len; url = ++j) {
          i = stun_servers[url];
          pc_config["iceServers"][i] = {
            "url": url
          };
        }
        this.pc = new RTCPeerConnection(pc_config);
        this.dc = this.pc.createDataChannel('data', {
          ordered: true,
          reliable: false
        });
        this.dc.onmessage = (function(_this) {
          return function(event) {
            return _this.ondcmessage(event);
          };
        })(this);
        this.pc.createOffer((function(_this) {
          return function(desc) {
            _this.pc.setLocalDescription(desc);
            return _this.signal_socket.send(JSON.stringify(desc));
          };
        })(this));
        this.signal_socket.setConductor(this);
      }

      Conductor.prototype.ondatachannel = function(event) {
        console.log(event);
        this.dc = event.channel;
        return this.dc.onmessage = function(event) {
          return console.log(event);
        };
      };

      Conductor.prototype.ondcmessage = function(event) {
        var message;
        message = JSON.parse(event.data);
        return console.log(message);
      };

      Conductor.prototype.onssmessage = function(event) {
        var message;
        message = JSON.parse(event.data);
        if (message.type === "answer") {
          return this.pc.setRemoteDescription(new RTCSessionDescription(message), (function(_this) {
            return function(event) {
              return console.log("Answer Set");
            };
          })(this));
        }
      };

      return Conductor;

    })();
    load_stun = function() {
      return $.get({
        url: document.location.origin + "/static/stun.txt",
        success: function(response) {
          return stun_servers = response.split("\n");
        }
      });
    };
    return $(function() {
      var signal_socket;
      load_stun();
      signal_socket = new SignalingSocket("ws://" + document.location.host + "/ws/device/test/");
      return $('#connect').on('click', (function(_this) {
        return function(e) {
          var conductor, i, j, len, pc_config, url;
          pc_config = {
            "iceServers": []
          };
          for (url = j = 0, len = stun_servers.length; j < len; url = ++j) {
            i = stun_servers[url];
            pc_config["iceServers"][i] = {
              "url": url
            };
          }
          return conductor = new Conductor(signal_socket);
        };
      })(this));
    });
  })(jQuery);

}).call(this);

//# sourceMappingURL=controls.js.map