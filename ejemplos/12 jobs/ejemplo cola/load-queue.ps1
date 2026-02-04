# hemos hecho previamente
# kubectl port-forward rs/queue 8080:8080

# Create a work queue called 'keygen'
curl.exe -X PUT gz.com/memq/server/queues/keygen

# Create 100 work items and load up the queue.
for i in work-item-{0..99}; do
  curl.exe -X POST gz.com/memq/server/queues/keygen/enqueue \
    -d "$i"
done
