#!/bin/env bash

INPUT_DIR="/raw_data"
OUTPUT_DIR="/processed_data"

mkdir -p "/db"

# only do so if the link doesn't exist
ln -sT /raw_data raw_data
ln -sT /processed_data processed_data
ln -sT /db db

find $INPUT_DIR -type f -not -path '*/.*' > ./all_files.txt

./epstein-file-review
