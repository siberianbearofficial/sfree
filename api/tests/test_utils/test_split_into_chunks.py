import pytest

from src.utils.split_into_chunks import split_into_chunks


def test_multiple_chunks_exact_size():
    content = b"a" * 3 + b"b" * 3
    chunk_size = 3
    chunks = list(split_into_chunks(content, chunk_size))
    assert len(chunks) == 2
    assert chunks[0] == b"aaa"
    assert chunks[1] == b"bbb"


def test_multiple_chunks_with_remainder():
    content = b"a" * 3 + b"b" * 4
    chunk_size = 3
    chunks = list(split_into_chunks(content, chunk_size))
    assert len(chunks) == 3
    assert chunks[0] == b"aaa"
    assert chunks[1] == b"bbb"
    assert chunks[2] == b"b"


def test_empty_content():
    content = b""
    chunk_size = 1024
    chunks = list(split_into_chunks(content, chunk_size))
    assert len(chunks) == 0


def test_chunk_size_larger_than_content():
    content = b"hello world"
    chunk_size = 100
    chunks = list(split_into_chunks(content, chunk_size))
    assert len(chunks) == 1
    assert chunks[0] == content


def test_chunk_size_equal_to_content():
    content = b"hello world"
    chunk_size = len(content)
    chunks = list(split_into_chunks(content, chunk_size))
    assert len(chunks) == 1
    assert chunks[0] == content


def test_chunk_size_zero():
    content = b"hello world"
    chunk_size = 0
    with pytest.raises(ValueError):
        list(split_into_chunks(content, chunk_size))


def test_negative_chunk_size():
    content = b"hello world"
    chunk_size = -1
    with pytest.raises(ValueError):
        list(split_into_chunks(content, chunk_size))
