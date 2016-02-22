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

  class Conductor
    constructor: (@signal_socket) ->
      pc_config = {"iceServers": []}
      for i, url in stun_servers
        pc_config["iceServers"][i] = {"url": url}

      @pc = new RTCPeerConnection(pc_config)

      @dc = @pc.createDataChannel('data', {ordered: true, reliable: false})
      @dc.onmessage = (event) =>
        @ondcmessage event

      @pc.createOffer (desc) =>
        @pc.setLocalDescription(desc)
        @signal_socket.send(JSON.stringify(desc))

      @signal_socket.setConductor(this)

    ondatachannel: (event) ->
      console.log(event)
      @dc = event.channel
      @dc.onmessage = (event) ->
        console.log(event)

    ondcmessage: (event) ->
      message = JSON.parse(event.data)
      console.log(message)

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

    $('#connect').on 'click', (e) =>
      pc_config = {"iceServers": []}
      for i, url in stun_servers
        pc_config["iceServers"][i] = {"url": url}
      conductor = new Conductor signal_socket

) jQuery