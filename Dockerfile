FROM alpine

ENV PAULING_PROFILER_ADDR=0.0.0.0:80
ADD pauling /pauling
ADD configs/configs /configs

ENTRYPOINT /pauling