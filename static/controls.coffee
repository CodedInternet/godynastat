(($) ->
  stun_servers = []
  sensors_ctx = document.getElementById("sensors").getContext("2d")

  class SignalingSocket
    constructor: (wsuri) ->
      @ws = new WebSocket(wsuri)
      @ws.onclose = @onclose

      @ws.onmessage = (event) =>
        @conductor.onssmessage(event)

    setConductor: (@conductor) ->

    onclose: (event) ->
      if(event.code >= event.code.CLOSE_PROTOCOL_ERROR)
        alert("Could not connect to signaling server")

    send: (msg) ->
      @ws.send(msg)


  class SensorSpot
    size = 10
    fade = 20

    constructor: (@x, @y) ->
      @value = 255
      @hue = 255.0

    setValue: (value) ->
      @value = 255 - value

    draw: (ctx) ->
      diff = @value - @hue
      if Math.abs(diff) < 0.5 # Helps ensure we have proper values
        @hue = @value
      else
        @hue += diff/fade

      gradient = ctx.createRadialGradient(@x, @y, 0.1, @x, @y, size)
      gradient.addColorStop(0, "hsla(#{@hue}, 80%, 50%, 1)")
      gradient.addColorStop(1, "hsla(#{@hue}, 80%, 50%, 0)")

      ctx.beginPath()
      ctx.arc(@x, @y, size, 0, 2 * Math.PI)
      ctx.fillStyle = gradient
      ctx.fill()
      ctx.closePath()

  class Dynastat
    @valid = false

    constructor: ->
      @canvas = document.getElementById("sensors")
      @ctx = @canvas.getContext("2d")
      @ctx.globalCompositeOperation = "lighter"
      @ctx.globalAlpha = 0.75

      @state = {
        "sensors": {
          "left_mtp": [],
          "right_mtp": [],
        },
        "motors": {}
      }

      for name, sensor of @state["sensors"]
        for row in [0..9]
          if !sensor[row]?
            sensor[row] = []
          for col in [0..15]
            size = 15
            cell = new SensorSpot( (size * col), (size * row))
            sensor[row][col] = cell
        @state["sensors"][name] = sensor

      setInterval(@draw.bind(this), 1000/30)

    updateState: (update) ->
      @updateSensors(update["sensors"])

    updateSensors: (update) ->
      for name, sensor of update
        for row, cols of sensor
          for col, value of cols
            cell = @state["sensors"][name][row][col]
            cell.setValue value
      @valid = false
      @draw()

    draw: ->
      @ctx.clearRect(0, 0, @canvas.width, @canvas.height)
      for name, sensor of @state["sensors"]
        for row, cols of sensor
          for col, cell of cols
            cell.draw @ctx

  class Conductor
    constructor: (@signal_socket) ->
      pc_config = {"iceServers": []}
      for i, url in stun_servers
        pc_config["iceServers"][i] = {"url": url}

      @pc = new RTCPeerConnection(pc_config)

      @rxDc = @pc.createDataChannel('data', {ordered: true, reliable: false})
      @rxDc.onmessage = (event) =>
        @ondcmessage event
#      @txDc = @pc.createDataChannel('command', {ordered: true, reliable: true})

      @signal_socket.setConductor(this)

      @device = new Dynastat

    open: ->
      @pc.createOffer (desc) =>
        @pc.setLocalDescription(desc)
        @signal_socket.send(JSON.stringify(desc))

    close: ->
      @pc.close()

    ondatachannel: (event) ->
      console.log(event)
      @dc = event.channel
      @dc.onmessage = (event) ->
        console.log(event)

    ondcmessage: (event) ->
      message = JSON.parse(event.data)
      @device.updateState message

    onssmessage: (event) ->
      message = JSON.parse(event.data)
      if (message.type == "answer")
        @pc.setRemoteDescription new RTCSessionDescription(message), (event) =>
          console.log("Answer Set")

  load_stun = ->
    $.get
      url: document.location.origin + "/static/stun.txt"
      success: (response) -> stun_servers = response.split("\n")

  $ -> # On ready
    load_stun()
    signal_socket = new SignalingSocket "ws://" + document.location.host + "/ws/device/test/"
    conductor = new Conductor signal_socket

    $('#connect').on 'click', (e) =>
      conductor.open()

) jQuery