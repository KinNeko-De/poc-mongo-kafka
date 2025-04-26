#!/bin/bash

docker compose -f sut/kafka.yaml up --build --remove-orphans --exit-code-from kafka