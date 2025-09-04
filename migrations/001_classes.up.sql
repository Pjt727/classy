-- notes:
-- can't use temp tables with sqlc so i use staging tables here
-- regex check constraints are mainly motiviated by api route names

-- migrate's drops will not delete types
DROP TYPE IF EXISTS season_enum;
CREATE TYPE season_enum AS ENUM ('Spring', 'Fall', 'Winter', 'Summer');
CREATE TYPE term_collection_status_enum AS ENUM ('Active', 'Success', 'Failure');

CREATE TABLE schools (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    CONSTRAINT id CHECK (id ~ '^[a-zA-Z]*$')
);

CREATE TABLE terms (
    year INT,
    season season_enum,

    PRIMARY KEY (year, season)
);

CREATE TABLE term_collections (
    id TEXT,
    school_id TEXT,

    year INT NOT NULL,
    season season_enum NOT NULL,
    name TEXT,
    still_collecting BOOL NOT NULL,
    FOREIGN KEY (school_id) REFERENCES schools(id),
    FOREIGN KEY (year, season) REFERENCES terms(year, season),
    PRIMARY KEY (id, school_id),
    CONSTRAINT id CHECK (id ~ '^[a-zA-Z0-9]*$')
);

CREATE TABLE term_collection_history (
    id SERIAL PRIMARY KEY,

    status term_collection_status_enum NOT NULL DEFAULT 'Active',
    term_collection_id TEXT NOT NULL,
    school_id TEXT NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    is_full BOOL NOT NULL,

    end_time TIMESTAMP WITH TIME ZONE,
    deleted_records_count INTEGER NOT NULL DEFAULT 0,
    updated_records_count INTEGER NOT NULL DEFAULT 0,
    inserted_records_count INTEGER NOT NULL DEFAULT 0,

    FOREIGN KEY (term_collection_id, school_id) REFERENCES term_collections(id, school_id)
);

-- ensure that there is only one active collection going on at once
CREATE UNIQUE INDEX ensure_unqiue_active_collection
ON term_collection_history (term_collection_id, school_id)
WHERE status = 'Active';

CREATE TABLE professors (
    id TEXT,
    school_id TEXT,

    name TEXT NOT NULL,
    email_address TEXT,
    first_name TEXT,
    last_name TEXT,
    other jsonb,
    FOREIGN KEY (school_id) REFERENCES schools(id),
    PRIMARY KEY (id, school_id)
);

-- TODO: test adding a HASH INDEX for term_collection_history_id for all staging tables
CREATE TABLE staging_professors (
    like professors
    including defaults,
    term_collection_history_id INT NOT NULL
);

CREATE TABLE courses (
    school_id TEXT,
    subject_code TEXT,
    number TEXT,

    subject_description TEXT,
    title TEXT,
    description TEXT,
    credit_hours REAL NOT NULL,
    prerequisites TEXT,
    corequisites TEXT,
    other jsonb,

    FOREIGN KEY (school_id) REFERENCES schools(id),
    PRIMARY KEY (school_id, subject_code, number),
    CONSTRAINT subject_code CHECK (subject_code ~ '^[a-zA-Z0-9]*$'),
    CONSTRAINT number CHECK (number ~ '^[a-zA-Z0-9]*$')
);

CREATE TABLE staging_courses (
    like courses
    including defaults,
    term_collection_history_id INT NOT NULL
);

CREATE TABLE sections (
    sequence TEXT,
    term_collection_id TEXT,
    subject_code TEXT,
    course_number TEXT,
    school_id TEXT,

    max_enrollment INTEGER,
    instruction_method TEXT,
    campus TEXT,
    enrollment INTEGER,
    primary_professor_id TEXT,
    other jsonb,
    FOREIGN KEY (school_id, subject_code, course_number) 
        REFERENCES courses(school_id, subject_code, number),
    FOREIGN KEY (primary_professor_id, school_id) REFERENCES professors(id, school_id),

    FOREIGN KEY (term_collection_id, school_id) REFERENCES term_collections(id, school_id),
    PRIMARY KEY (sequence, term_collection_id, subject_code, course_number, school_id),
    CONSTRAINT subject_code CHECK (sequence ~ '^[a-zA-Z0-9]*$')
);

CREATE TABLE staging_sections (
    like sections
    including defaults,
    term_collection_history_id INT NOT NULL
);


CREATE TABLE meeting_times (
    sequence INT,
    section_sequence TEXT,
    term_collection_id TEXT,
    subject_code TEXT,
    course_number TEXT,
    school_id TEXT,

    start_date TIMESTAMP,
    end_date TIMESTAMP,
    meeting_type TEXT,
    start_minutes TIME,
    end_minutes TIME,
    is_monday BOOLEAN NOT NULL,
    is_tuesday BOOLEAN NOT NULL,
    is_wednesday BOOLEAN NOT NULL,
    is_thursday BOOLEAN NOT NULL,
    is_friday BOOLEAN NOT NULL,
    is_saturday BOOLEAN NOT NULL,
    is_sunday BOOLEAN NOT NULL,
    other jsonb,

    FOREIGN KEY (section_sequence, term_collection_id, school_id, subject_code, course_number)
        REFERENCES sections(sequence, term_collection_id, school_id, subject_code, course_number) ON DELETE CASCADE,
    PRIMARY KEY (sequence, section_sequence, term_collection_id, subject_code, course_number, school_id)
);

CREATE TABLE staging_meeting_times (
    like meeting_times
    including defaults,
    term_collection_history_id INT NOT NULL
);

