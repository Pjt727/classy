DROP TRIGGER IF EXISTS historic_log ON term_collections;
DROP TRIGGER IF EXISTS historic_log ON professors;
DROP TRIGGER IF EXISTS historic_log ON courses;
DROP TRIGGER IF EXISTS historic_log ON sections;
DROP TRIGGER IF EXISTS historic_log ON meeting_times;
DROP TRIGGER IF EXISTS historic_log ON schools;

DROP FUNCTION IF EXISTS log_historic_class_information();
