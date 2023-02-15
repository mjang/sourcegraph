# This image creates a standalone web server for demoing the prototype
FROM node:16@sha256:ce3d204ff857d48fbf2cdd16649b5a0b57cc1fd1a3cb2ff0d8727532f5211cf1

ARG PROJECT_ROOT="."

WORKDIR /usr/src/app
RUN echo '{"type": "module"}' > package.json
RUN npm install express@~4.18 http-proxy-middleware@~2.0
COPY  ${PROJECT_ROOT}/server.js ./
COPY ${PROJECT_ROOT}/build ./build

ENV SOURCEGRAPH_API_HOST=https://sourcegraph.com
EXPOSE 4173
CMD ["node", "server.js"]
