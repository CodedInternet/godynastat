(($) ->
  stun_servers = []

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

  class Dynastat
    constructor: ->
      @state = {
        "sensors": {},
        "motors": {}
      }

    updateState: (update) ->
      @updateSensors(update["sensors"])

    updateSensors: (update) ->
      for name, sensor of update
        @state["sensors"][name] = state = [sensor.length]
        for row, cols of sensor
          state[row] = [cols.length]
          for col, value of cols
            $element = state[row][col]
            if !$element? or !$element.data?
              id = "#{name}_#{row}_#{col}"
              $element = $("##{id}")
              state[row][col] = $element

            hue = 255 - value
            $element.css("background", "hsl(#{hue}, 80%, 50%)")
            $element.data("value", value)

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

      @pc.createOffer (desc) =>
        @pc.setLocalDescription(desc)
        @signal_socket.send(JSON.stringify(desc))

      @signal_socket.setConductor(this)

      @device = new Dynastat

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
    conductor = undefined

    $('#connect').on 'click', (e) =>
      if conductor?
        conductor.close()
        conductor = undefined
      else
        conductor = new Conductor signal_socket

) jQuery