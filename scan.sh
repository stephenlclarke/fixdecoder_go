#!/bin/bash

 docker run --rm -e SONAR_TOKEN="${SONAR_TOKEN}" -v "$(pwd):/usr/src" sonarsource/sonar-scanner-cli
