DROP TRIGGER IF EXISTS term_collections_trigger ON term_collections;
DROP TRIGGER IF EXISTS professors_trigger ON professors;
DROP TRIGGER IF EXISTS courses_trigger ON courses;
DROP TRIGGER IF EXISTS sections_trigger ON sections;
DROP TRIGGER IF EXISTS meeting_times_trigger ON meeting_times;

DROP FUNCTION IF EXISTS log_historic_class_information();
