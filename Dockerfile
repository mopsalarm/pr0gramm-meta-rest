FROM centurylink/ca-certs
EXPOSE 8080
COPY pr0gramm-meta-rest /
ENTRYPOINT ["/pr0gramm-meta-rest"]
