#!/bin/bash

param="$1"
docker compose -f sut/kafka.yaml -f sut/mongodb.yaml up $param