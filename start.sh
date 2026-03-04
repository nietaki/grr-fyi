#!/bin/env bash

INPUT_DIR="/raw_data"
OUTPUT_DIR="/processed_data"

ln -s /raw_data ./raw_data
ln -s /processed_data ./processed_data

mkdir -p "/db"
find $INPUT_DIR -type f -not -path '*/.*' > ./all_files.txt

./epstein-file-review
