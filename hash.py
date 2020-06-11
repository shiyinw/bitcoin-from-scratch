import hashlib


def get_hash_string(input_str: str):
    return get_hash_bytes(input_str).hex()


def get_hash_bytes(input_str: str):
    m = hashlib.sha256()
    m.update(input_str.encode())
    return m.digest()


def check_hash(input_hash: str):
    return input_hash[:5] == "00000"
