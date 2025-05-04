#!/bin/bash

docker compose -p cluster -f sut/kafka.yaml -f sut/mongodb.yaml down --volumes --remove-orphans