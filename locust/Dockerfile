FROM locustio/locust:2.32.0
COPY . /mnt/locust
CMD [ "-f", "/mnt/locust/locustfile.py", "--host", "http://api:8080"]