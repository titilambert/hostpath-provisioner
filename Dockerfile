FROM debian

ADD hostpath-provisioner /hostpath-provisioner

ENTRYPOINT /hostpath-provisioner
