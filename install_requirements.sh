#!/bin/bash

usage() {
  echo "Usage: $0 [--target DIR] | [--stdin]"
  exit 1
}

if [[ $# -eq 0 ]]; then
  # Сценарий 1: Нет аргументов, установка в директорию по умолчанию
  pip install --no-cache-dir -r requirements.txt
  LOCAL_LIBS=$(ls *.whl 2>/dev/null)
  if [[ -n "$LOCAL_LIBS" ]]; then
    pip install --no-cache-dir $LOCAL_LIBS
  fi
elif [[ $# -eq 2 && $1 == "--target" ]]; then
  # Сценарий 2: Установка в заданную директорию DIR
  TARGET_DIR=$2
  pip install --no-cache-dir --target="$TARGET_DIR" -r requirements.txt
  LOCAL_LIBS=$(ls *.whl 2>/dev/null)
  if [[ -n "$LOCAL_LIBS" ]]; then
    pip install --no-cache-dir --target="$TARGET_DIR" $LOCAL_LIBS
  fi
elif [[ $# -eq 1 && $1 == "--stdin" ]]; then
  # Сценарий 3: Директория из входного потока
  read -r TARGET_DIR
  if [[ -z "$TARGET_DIR" ]]; then
    echo "Error: No directory provided via stdin." >&2
    exit 1
  fi
  pip install --no-cache-dir --target="$TARGET_DIR" -r requirements.txt
  LOCAL_LIBS=$(ls *.whl 2>/dev/null)
  if [[ -n "$LOCAL_LIBS" ]]; then
    pip install --no-cache-dir --target="$TARGET_DIR" $LOCAL_LIBS
  fi
else
  # Сценарий 4: Некорректные аргументы
  usage
fi
