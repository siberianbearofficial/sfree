ARG PYTHON_IMAGE=python:3.12-slim
FROM ${PYTHON_IMAGE}

WORKDIR /work
COPY e2e/requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt
COPY e2e /work/e2e
COPY e2e/pytest.ini /work
ENV PYTHONPATH=/work
CMD ["pytest", "-q", "e2e/test_api_e2e.py"]
