#!/bin/env bash

ln -s /raw_data ./raw_data
ln -s /processed_data ./processed_data

# this script converts all (nested) .flac files from /input to opus files in /output
INPUT_DIR="/raw_data"
OUTPUT_DIR="/processed_data"

FILE_COUNT=$(find "$INPUT_DIR" -type f -iname "*.pdf" | wc -l)
echo "Found $FILE_COUNT PDF files to convert."

find $INPUT_DIR -type f  -not -path '*/.*' | sed 's/.*\.//' | sort | uniq -c
find $INPUT_DIR -type f  -not -path '*/.*' > ./all_files.txt

FILE_NO=0


# find "$INPUT_DIR" -type f -iname "*.pdf" | while read -r PDF_FILE; do
#     # Determine the relative path of the FLAC file with respect to the input directory

#     FILE_NO=$((FILE_NO + 1))
#     # every 100 files, print progress
#     if (( FILE_NO % 1000 == 0 )); then
#         PERCENT=$(( FILE_NO * 100 / FILE_COUNT ))
#         echo "$PERCENT%: $FILE_NO / $FILE_COUNT files."
#         # print progress percentage
#     fi

#     RELATIVE_PATH="${PDF_FILE#$INPUT_DIR/}"
    
#     # Determine the output directory and create it if it doesn't exist
#     OUTPUT_SUBDIR="$(dirname "$OUTPUT_DIR/$RELATIVE_PATH")"
#     mkdir -p "$OUTPUT_SUBDIR"
    
#     # Determine the output file name by replacing .flac with .opus
#     OUTPUT_FILE="$OUTPUT_SUBDIR/$(basename "${RELATIVE_PATH}.content.txt")"

#     # check if the file already exists
#     if [ -f "$OUTPUT_FILE" ]; then
#         # echo "Skipping '$PDF_FILE' as '$OUTPUT_FILE' already exists."
#         continue
#     fi

#     # echo "\nConverting '$PDF_FILE' to '$OUTPUT_FILE'"
#     pdftotext "$PDF_FILE" "$OUTPUT_FILE"
# done


echo "we're out"
