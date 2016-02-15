FROM alpine

ADD pauling /pauling
ADD configs /configs
ENV PAULING_DOCKER=true

ENTRYPOINT /pauling