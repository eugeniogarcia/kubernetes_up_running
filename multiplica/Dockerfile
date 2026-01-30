FROM node:18-alpine

WORKDIR /app

COPY package.json ./

# Default multiplier (can be overridden when creating the Pod)
ENV MULTIPLIER=2

RUN npm install --production

COPY app.js ./

EXPOSE 8080

CMD ["node", "app.js"]
