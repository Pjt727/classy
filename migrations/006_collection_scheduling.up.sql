CREATE EXTENSION IF NOT EXISTS pgmq;

SELECT pgmq.create('collection_jobs');
