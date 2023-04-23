ARG GITLAB_RUNNER_IMAGE_TAG
FROM golang:1.20.1-alpine AS builder

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/giruno

FROM gitlab/gitlab-runner:${GITLAB_RUNNER_IMAGE_TAG}

COPY --from=builder /usr/local/bin/giruno /usr/local/bin/giruno
