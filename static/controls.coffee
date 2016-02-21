(($) ->
  data_channels = []
  stun_servers = []

  load_stun = ->
    $.get
      url: document.location.origin + "/static/stun.txt"
      success: (response) -> stun_servers = response.split("\n")

  $ -> # On ready
    load_stun()

) jQuery