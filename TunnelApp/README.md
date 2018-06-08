# TunnelApp
A simple boot app able to stand in for a CF deployed app and act as a host for ssh tunnels.

There isn't much to this app. Basically it is deployed to PCF and pretends to be an app but
requests that are made to it (once appropriate tunnels have been setup) are routed elsewhere,
usually to a process on the developers local machine.  The aim is that this app is built and
then deployed using the tunnel-boot CF CLI plugin. 

TunnelApp can stand in for any app, it is the yml customization that tweaks it a little
to stand in for a specific app.
