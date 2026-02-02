FROM seangreenhalgh/nanit:latest

USER root

COPY run.sh /run.sh
RUN chmod a+x /run.sh

ENTRYPOINT ["/run.sh"]
