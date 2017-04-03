// Generated by CoffeeScript 1.12.4
(function() {
  (function($) {
    var Conductor, Dynastat, SensorSpot, SignalingSocket, count, fps, load_stun, stepPrecision, stun_servers;
      stun_servers = ['stun.stunprotocol.org'];
    fps = 0;
    count = 0;
    Number.prototype.map = function(in_min, in_max, out_min, out_max) {
      return (this - in_min) * (out_max - out_min) / (in_max - in_min) + out_min;
    };
    stepPrecision = function($input) {
      var parts, precision, step;
      step = $input.attr('step');
      parts = (step + "").split(".");
      precision = 0;
      if (parts[1] != null) {
        precision += parts[1].length;
      }
      return precision;
    };
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
      var fade;

      SensorSpot.size = 10;

      fade = 5;

      function SensorSpot(x1, y1) {
        this.x = x1;
        this.y = y1;
        this.value = 255;
        this.hue = 255.0;
      }

      SensorSpot.prototype.setValue = function(value) {
        return this.value = 255 - value;
      };

      SensorSpot.prototype.draw = function(ctx) {
        var diff, gradient, size;
        diff = this.value - this.hue;
        if (Math.abs(diff) < 0.5) {
          this.hue = this.value;
        } else {
          this.hue += diff / fade;
        }
        size = this.constructor.size;
        gradient = ctx.createRadialGradient(this.x, this.y, 0.1, this.x, this.y, size - size / 3);
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
        var cell, col, conf, config, j, k, name, ref, ref1, ref2, row, sensor, size, x, y;
        this.canvas = document.getElementById("sensors");
        this.ctx = this.canvas.getContext("2d");
        this.ctx.globalCompositeOperation = "lighter";
        this.ctx.globalAlpha = 0.75;
        config = {
          "sensors": {
            "left_mtp": {
              "x": 50,
              "y": 110,
              "rows": 10,
              "cols": 16
            },
            "left_hallux": {
              "x": 215,
              "y": 100,
              "rows": 12,
              "cols": 6
            },
            "left_heel": {
              "x": 150,
              "y": 360,
              "rows": 12,
              "cols": 12
            },
            "right_mtp": {
              "x": 400,
              "y": 110,
              "rows": 10,
              "cols": 16
            },
            "right_hallux": {
              "x": 335,
              "y": 100,
              "rows": 12,
              "cols": 6
            },
            "right_heel": {
              "x": 345,
              "y": 360,
              "rows": 12,
              "cols": 12
            }
          },
          "motors": {}
        };
        this.state = {
          "sensors": {},
          "motors": {}
        };
        ref = config["sensors"];
        for (name in ref) {
          conf = ref[name];
          sensor = this.state["sensors"][name];
          sensor = [];
          for (row = j = 0, ref1 = conf["rows"] - 1; 0 <= ref1 ? j <= ref1 : j >= ref1; row = 0 <= ref1 ? ++j : --j) {
            if (sensor[row] == null) {
              sensor[row] = [];
            }
            for (col = k = 0, ref2 = conf["cols"] - 1; 0 <= ref2 ? k <= ref2 : k >= ref2; col = 0 <= ref2 ? ++k : --k) {
              size = new SensorSpot().constructor.size;
              x = size * col + conf.x;
              y = size * row + conf.y;
              cell = new SensorSpot(x, y);
              sensor[row][col] = cell;
            }
          }
          this.state["sensors"][name] = sensor;
        }
        requestAnimationFrame(this.draw.bind(this));
      }

      Dynastat.prototype.updateState = function(update) {
          console.log(update);
          this.updateSensors(update["Sensors"]);
          return this.updateMotors(update["Motors"]);
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

      Dynastat.prototype.updateMotors = function(update) {
        var $input, $output, current, id, max, min, motor, name, results, target;
        results = [];
        for (name in update) {
          motor = update[name];
          id = "#m_" + name;
            target = Number(motor["Target"]);
            current = Number(motor["Current"]);
          $input = $(id);
          $output = $(id + "_current");
          min = Number($input.attr('min'));
          max = Number($input.attr('max'));
          results.push($output.val(current.map(0, 255, min, max).toFixed(stepPrecision($input))));
        }
        return results;
      };

      Dynastat.prototype.draw = function() {
        var cell, col, cols, name, ref, results, row, sensor;
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        ref = this.state["sensors"];
        results = [];
        for (name in ref) {
          sensor = ref[name];
          results.push((function() {
            var results1;
            results1 = [];
            for (row in sensor) {
              cols = sensor[row];
              results1.push((function() {
                var results2;
                results2 = [];
                for (col in cols) {
                  cell = cols[col];
                  results2.push(cell.draw(this.ctx));
                }
                return results2;
              }).call(this));
            }
            return results1;
          }).call(this));
        }
        return results;
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
        this.rxDc.binaryType = "arraybuffer";
        this.rxDc.onmessage = (function(_this) {
          return function(event) {
            return _this.ondcmessage(event);
          };
        })(this);
        this.txDc = this.pc.createDataChannel('command', {
          ordered: true,
          reliable: true
        });
        this.signal_socket.setConductor(this);
        this.device = new Dynastat;
      }

      Conductor.prototype.open = function() {
        return this.pc.createOffer(((function(_this) {
          return function(desc) {
            _this.pc.setLocalDescription(desc);
            return _this.signal_socket.send(JSON.stringify(desc));
          };
        })(this)), ((function(_this) {
          return function() {
            return console.log("Create Offer failed");
          };
        })(this)));
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
        var data, state;
        fps++;
        data = new Uint8Array(event.data);
        state = msgpack.unpack(data);
        return this.device.updateState(state);
      };

      Conductor.prototype.onssmessage = function(event) {
        var message;
        message = JSON.parse(event.data);
        if (message.type === "answer") {
          return this.pc.setRemoteDescription(new RTCSessionDescription(message), (function(_this) {
            return function(event) {
              return $('.m_input').removeAttr('disabled');
            };
          })(this));
        }
      };

      Conductor.prototype.setmotor = function(name, value) {
        var json;
        json = JSON.stringify({
          "cmd": "set_motor",
          "name": name,
          "value": "value",
          value: value
        });
        console.log(json);
        return this.txDc.send(json);
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
      var conductor, signal_socket, update_fps;
      load_stun();
      signal_socket = new SignalingSocket("ws://" + document.location.host + "/ws/device/test/");
      conductor = new Conductor(signal_socket);
      $.conductor = conductor;
      $('#connect').on('click', (function(_this) {
        return function(e) {
          return conductor.open();
        };
      })(this));
      update_fps = function() {
        $('#fps').text(fps);
        return fps = 0;
      };
      setInterval(update_fps, 1000);
      return $('.m_input').on('change', function() {
        var max, min, name, val;
        name = $(this).attr('name');
        min = $(this).attr('min');
        max = $(this).attr('max');
        val = Number($(this).val());
        return $.conductor.setmotor(name, Math.round(val.map(min, max, 0, 255)));
      });
    });
  })(jQuery);

}).call(this);
