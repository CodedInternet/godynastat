// Generated by CoffeeScript 1.10.0
(function() {
  var Conductor, Dynastat, SensorSpot, SignalingSocket, load_stun, sensors_ctx, stun_servers;

  stun_servers = [];

  sensors_ctx = document.getElementById("sensors").getContext("2d");

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

  SensorSpot = (function() {
    var size;

    size = 15;

    function SensorSpot(x, y) {
      this.x = x;
      this.y = y;
      this.value = 0;
      this.hue = 255;
    }

    SensorSpot.prototype.setValue = function(value1) {
      this.value = value1;
      return this.hue = 255 - this.value;
    };

    SensorSpot.prototype.draw = function(ctx) {
      var gradient;
      gradient = ctx.createRadialGradient(this.x, this.y, 0, this.x, this.y, size);
      gradient.addColorStop(0, "hsla(" + this.hue + ", 80%, 50%, 1)");
      gradient.addColorStop(1, "hsla(" + this.hue + ", 80%, 50%, 0)");
      ctx.beginPath();
      ctx.arc(this.x, this.y, size, 0, 2 * Math.PI);
      ctx.fillStyle = gradient;
      ctx.fill();
      return ctx.closePath();
    };

    return SensorSpot;

  })();

  Dynastat = (function() {
    Dynastat.valid = false;

    function Dynastat() {
      var cell, col, j, k, name, ref, row, sensor, size;
      this.ctx = document.getElementById("sensors").getContext("2d");
      this.state = {
        "sensors": {
          "left_mtp": [],
          "right_mtp": []
        },
        "motors": {}
      };
      ref = this.state["sensors"];
      for (name in ref) {
        sensor = ref[name];
        for (row = j = 0; j <= 9; row = ++j) {
          if (sensor[row] == null) {
            sensor[row] = [];
          }
          for (col = k = 0; k <= 15; col = ++k) {
            size = 15;
            cell = new SensorSpot(size * col, size * row);
            sensor[row][col] = cell;
          }
        }
        this.state["sensors"][name] = sensor;
      }
      setInterval(this.draw.bind(this), 30);
    }

    Dynastat.prototype.updateState = function(update) {
      return this.updateSensors(update["sensors"]);
    };

    Dynastat.prototype.updateSensors = function(update) {
      var cell, col, cols, name, row, sensor, value;
      for (name in update) {
        sensor = update[name];
        for (row in sensor) {
          cols = sensor[row];
          for (col in cols) {
            value = cols[col];
            cell = this.state["sensors"][name][row][col];
            cell.setValue(value);
          }
        }
      }
      this.valid = false;
      return this.draw();
    };

    Dynastat.prototype.draw = function() {
      var cell, col, cols, name, ref, row, sensor;
      if (!this.valid) {
        ref = this.state["sensors"];
        for (name in ref) {
          sensor = ref[name];
          for (row in sensor) {
            cols = sensor[row];
            for (col in cols) {
              cell = cols[col];
              cell.draw(this.ctx);
            }
          }
        }
        return this.valid = true;
      }
    };

    return Dynastat;

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
      this.rxDc = this.pc.createDataChannel('data', {
        ordered: true,
        reliable: false
      });
      this.rxDc.onmessage = (function(_this) {
        return function(event) {
          return _this.ondcmessage(event);
        };
      })(this);
      this.signal_socket.setConductor(this);
      this.device = new Dynastat;
    }

    Conductor.prototype.open = function() {
      return this.pc.createOffer((function(_this) {
        return function(desc) {
          _this.pc.setLocalDescription(desc);
          return _this.signal_socket.send(JSON.stringify(desc));
        };
      })(this));
    };

    Conductor.prototype.close = function() {
      return this.pc.close();
    };

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
      return this.device.updateState(message);
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

  $(function() {
    var conductor, signal_socket;
    load_stun();
    signal_socket = new SignalingSocket("ws://" + document.location.host + "/ws/device/test/");
    conductor = new Conductor(signal_socket);
    return $('#connect').on('click', (function(_this) {
      return function(e) {
        return conductor.open();
      };
    })(this));
  });

}).call(this);

//# sourceMappingURL=controls.js.map
