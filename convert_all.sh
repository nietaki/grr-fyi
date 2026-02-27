#!/bin/env bash

INPUT_DIR="/raw_data"
OUTPUT_DIR="/processed_data"

find $INPUT_DIR -type f -not -path '*/.*' | sed 's/.*\.//' | sort | uniq -c

FILE_NO=0

find "$INPUT_DIR" -type f -not -path '*/.*'  -iname "*.pdf" | while read -r WAV_FILE; do
    FILE_NO=$((FILE_NO + 1))
    echo "mp3: $FILE_NO file."

    RELATIVE_PATH="${WAV_FILE#$INPUT_DIR/}"
    
    # Determine the output directory and create it if it doesn't exist
    OUTPUT_SUBDIR="$(dirname "$OUTPUT_DIR/$RELATIVE_PATH")"
    mkdir -p "$OUTPUT_SUBDIR"
    
    OUTPUT_FILE="$OUTPUT_SUBDIR/$(basename "${RELATIVE_PATH}.ogg")"

    # check if the file already exists
    if [ -f "$OUTPUT_FILE" ]; then
        echo "Skipping '$WAV_FILE' as '$OUTPUT_FILE' already exists."
        continue
    fi

    echo "\nConverting '$WAV_FILE' to '$OUTPUT_FILE'"
    ffmpeg -i "$WAV_FILE" "$OUTPUT_FILE"
done

FILE_COUNT=$(find "$INPUT_DIR" -type f -iname "*.pdf" | wc -l)
echo "Found $FILE_COUNT PDF files to convert."

FILE_NO=0

find "$INPUT_DIR" -type f -not -path '*/.*'  -iname "*.pdf" | while read -r PDF_FILE; do
    # Determine the relative path of the FLAC file with respect to the input directory

    FILE_NO=$((FILE_NO + 1))
    # every 100 files, print progress
    if (( FILE_NO % 1000 == 0 )); then
        PERCENT=$(( FILE_NO * 100 / FILE_COUNT ))
        echo "$PERCENT%: $FILE_NO / $FILE_COUNT files."
        # print progress percentage
    fi

    RELATIVE_PATH="${PDF_FILE#$INPUT_DIR/}"
    
    # Determine the output directory and create it if it doesn't exist
    OUTPUT_SUBDIR="$(dirname "$OUTPUT_DIR/$RELATIVE_PATH")"
    mkdir -p "$OUTPUT_SUBDIR"
    
    OUTPUT_FILE="$OUTPUT_SUBDIR/$(basename "${RELATIVE_PATH}.content.txt")"

    # check if the file already exists
    if [ -f "$OUTPUT_FILE" ]; then
        # echo "Skipping '$PDF_FILE' as '$OUTPUT_FILE' already exists."
        continue
    fi

    # echo "\nConverting '$PDF_FILE' to '$OUTPUT_FILE'"
    pdftotext "$PDF_FILE" "$OUTPUT_FILE"
done


echo "we're out"
