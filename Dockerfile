FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-atlassian"]
COPY baton-atlassian /