FROM postgres:18-alpine

COPY /ironValeInit /docker-entrypoint-initdb.d/
