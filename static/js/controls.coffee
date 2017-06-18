(($) ->
  stun_servers = [
    'stun.stunprotocol.org'
  ]
  fps = 0
  count = 0

  Number::map = (in_min, in_max, out_min, out_max) ->
    return (this - in_min) * (out_max - out_min) / (in_max - in_min) + out_min

  stepPrecision = ($input) ->
    step = $input.attr 'step'
    parts = (step + "").split(".")
    precision = 0
    if parts[1]?
      precision += parts[1].length
    return precision

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
    @size = 10
    fade = 5

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

      size = @constructor.size
      gradient = ctx.createRadialGradient(@x, @y, 0.1, @x, @y, size-size/3)
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

      config = {
        "sensors": {
          "left_mtp": {
            "x": 50,
            "y": 110,
            "rows": 10,
            "cols": 16,
          },
          "left_hallux": {
            "x": 215,
            "y": 100,
            "rows": 12,
            "cols": 6,
          },
          "left_heel": {
            "x": 150,
            "y": 360,
            "rows": 12,
            "cols": 12,
          },
          "right_mtp": {
            "x": 400,
            "y": 110,
            "rows": 10,
            "cols": 16,
          },
          "right_hallux": {
            "x": 335,
            "y": 100,
            "rows": 12,
            "cols": 6,
          },
          "right_heel": {
            "x": 345,
            "y": 360,
            "rows": 12,
            "cols": 12,
          },
        },
        "motors": {},
      }

      @state = {
        "sensors": {},
        "motors": {}
      }

      for name, conf of config["sensors"]
        sensor = @state["sensors"][name]
        sensor = []
        for row in [0..(conf["rows"]-1)]
          if !sensor[row]?
            sensor[row] = []
          for col in [0..(conf["cols"]-1)]
            size = new SensorSpot().constructor.size
            x = size * col + conf.x
            y = size * row + conf.y
            cell = new SensorSpot x, y
            sensor[row][col] = cell
        @state["sensors"][name] = sensor

      requestAnimationFrame(@draw.bind(this))

    updateState: (update) ->
      console.log(update)
      @updateSensors(update["Sensors"])
      @updateMotors(update["Motors"])

    updateSensors: (update) ->
      for name, sensor of update
        for row, cols of sensor
          for col, value of cols
            cell = @state["sensors"][name][row][col]
            cell.setValue value
      @valid = false
      @draw()

    updateMotors: (update) ->
      for name, motor of update
        id = "#m_#{name}"
        target = Number motor["Target"]
        current = Number motor["Current"]

        $input = $(id)
        $output = $(id+"_current")

        min = Number $input.attr 'min'
        max = Number $input.attr 'max'

#        $input.val target.map 0, 255, min, max
        $output.val current.map(0, 255, min, max).toFixed(stepPrecision $input)

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
      @rxDc.binaryType = "arraybuffer"
      @rxDc.onmessage = (event) =>
        @ondcmessage event
      @txDc = @pc.createDataChannel('command', {ordered: true, reliable: true})

      @signal_socket.setConductor(this)

      @device = new Dynastat

    open: ->
      @pc.createOffer \
        ((desc) =>
          @pc.setLocalDescription(desc)
          @signal_socket.send(JSON.stringify(desc)))
        ,
        (() =>
          console.log("Create Offer failed");
        )


    close: ->
      @pc.close()

    ondatachannel: (event) ->
      console.log(event)
      @dc = event.channel
      @dc.onmessage = (event) ->
        console.log(event)

    ondcmessage: (event) ->
      fps++
      data = new Uint8Array event.data
      state = msgpack.unpack data
      @device.updateState state

    onssmessage: (event) ->
      message = JSON.parse(event.data)
      console.log(message)
      if (message.type == "answer")
        @pc.setRemoteDescription new RTCSessionDescription(message), (event) =>
          $('.m_input').removeAttr 'disabled'
      else if (message.candidate)
        @pc.addIceCandidate(new RTCIceCandidate(message))

    setmotor: (name, value) ->
      json = JSON.stringify {"cmd": "set_motor", "name": name, "value", value}
      console.log json
      @txDc.send json

  load_stun = ->
    $.get
      url: document.location.origin + "/static/stun.txt"
      success: (response) -> stun_servers = response.split("\n")

  $ -> # On ready
    load_stun()
    signal_socket = new SignalingSocket "ws://" + document.location.host + "/ws/device/test/"
    conductor = new Conductor signal_socket
    $.conductor = conductor

    $('#connect').on 'click', (e) =>
      conductor.open()

    update_fps = ->
      $('#fps').text fps
      fps = 0

    setInterval update_fps, 1000

    $('.m_input').on 'change', ->
      # Actually send this data to the device and get a response
# @todo: Make sure the input displays correctly
      name = $(this).attr('name')
      min = $(this).attr('min')
      max = $(this).attr('max')
      val = Number $(this).val()
      $.conductor.setmotor name, Math.round val.map min, max, 0, 255

) jQuery
