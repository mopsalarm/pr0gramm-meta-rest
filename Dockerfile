FROM centurylink/ca-certs
ENV GOMAXPROCS 4
EXPOSE 8080
COPY pr0gramm-meta-rest /
ENTRYPOINT ["/pr0gramm-meta-rest"]
