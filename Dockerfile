FROM scratch

COPY grinklers config.json /

ENTRYPOINT ["/grinklers"]
