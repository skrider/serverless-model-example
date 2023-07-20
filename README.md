# Serverless Model-As-A-Service Example

How to run:

```sh
# confirm that the directory is titled "serverless-model-example" for docker-compose
basename $(pwd)
# serverless-model-example

docker --version
# Docker version 20.10.21, build baeda1f

docker compose build model-manual
# ...
# => exporting to image                                                                               0.0s
# => => exporting layers                                                                              0.0s
# => => writing image sha256:9ad7adf29a49d4d714fed5375c70d2c54bd3d5afc35ea151f0a39e125315a1c7         0.0s
# => => naming to IMAGE_NAME                                                                          0.0s
nano docker-compose.yml
# verify APP_REPLICA_IMAGE is the IMAGE_NAME from above, probably docker.io/library/serverless-model-example-model-manual

# to run docker tests:
docker compose up --build test_scheduler

# to run the service:
docker compose up --build scheduler

# to run test:
pip install requests
python tests/main.py
```

## `model`

This folder contains scaffolding for a client's model. `Dockerfile` specifies how to build the image, and `model.py` contains the client's model. `server.py` is a basic Flask server that serves the model on port 8000 of the container. While the client is free to modify either, or to use a completely different language, the scheduler expects all `model` instances to follow the same API - `POST /` for inference, and `GET /ok` for status.

Each replica of the model spun up by `scheduler` corresponds to one container.

## `scheduler`

This folder contains code for the scheduler. 

`replica.go` contains code for using the Docker SDK for managing model replicas. `replica_test.go` contains unit tests for this code. `replica_test.go` should only be run inside a container via `docker compose up --build test_scheduler`.

`job.go` contains the type definition and some utility functions for jobs.

`metrics.go` defines `MovingAverageDuration`, an object used to maintain an average of time taken to spin up models and handle requests.

`main.go` contains code for the web server. A high level overview of its architecture is as follows.

- `http.ListenAndServe` listens for new requests, queries the state of the `jobs` mapping, and queues jobs to the central `jobQueue` to be handled
- `backgroundPersistenceWorker` periodically writes `jobs` to a docker volume for persistence
- `backgroundJobWorker` de-queues jobs from the `jobQueue` and assigns them to the next-available `Replica`, sometimes spinning up new replicas
- `backgroundReplicaWorker` manages the life-cycle of one particular `Replica`, stopping it when it has exhausted its job queue. It is the responsibility of `backgroundJobWorker` to ensure that each replica's queue is saturated optimally.

All the rest of `main.go` is just pretty standard HTTP server boilerplate.

