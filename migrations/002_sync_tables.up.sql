DROP TYPE IF EXISTS sync_kind;
CREATE TYPE sync_kind AS ENUM ('update', 'delete', 'insert');

CREATE TABLE historic_term_collections (
    input_at TIMESTAMP WITH TIME ZONE,
    id TEXT,
    school_id TEXT,

    sync_action sync_kind,

    year INT,
    season season_enum,
    name TEXT,
    still_collecting BOOL,
    PRIMARY KEY (input_at, id, school_id)
);

CREATE TABLE historic_professors (
    input_at TIMESTAMP WITH TIME ZONE,
    id TEXT,
    school_id TEXT,

    sync_action sync_kind,

    name TEXT,
    email_address TEXT,
    first_name TEXT,
    last_name TEXT,
    PRIMARY KEY (input_at, id, school_id)
);

CREATE TABLE historic_courses (
    input_at TIMESTAMP WITH TIME ZONE,
    school_id TEXT,
    subject_code TEXT,
    number TEXT,

    sync_action sync_kind,

    subject_description TEXT,
    title TEXT,
    description TEXT,
    credit_hours REAL,
    PRIMARY KEY (input_at, school_id, subject_code, number)
);

CREATE TABLE historic_sections (
    input_at TIMESTAMP WITH TIME ZONE,
    sequence TEXT,
    term_collection_id TEXT,
    subject_code TEXT,
    course_number TEXT,
    school_id TEXT,

    sync_action sync_kind,

    max_enrollment INTEGER,
    instruction_method TEXT,
    campus TEXT,
    enrollment INTEGER,
    primary_professor_id TEXT,
    PRIMARY KEY (input_at, sequence, term_collection_id, subject_code, course_number, school_id)
);

CREATE TABLE historic_meeting_times (
    input_at TIMESTAMP WITH TIME ZONE,
    sequence INT,
    section_sequence TEXT,
    term_collection_id TEXT,
    subject_code TEXT,
    course_number TEXT,
    school_id TEXT,

    sync_action sync_kind,

    start_date TIMESTAMP,
    end_date TIMESTAMP,
    meeting_type TEXT,
    start_minutes TIME,
    end_minutes TIME,
    is_monday BOOLEAN,
    is_tuesday BOOLEAN,
    is_wednesday BOOLEAN,
    is_thursday BOOLEAN,
    is_friday BOOLEAN,
    is_saturday BOOLEAN,
    is_sunday BOOLEAN,
    PRIMARY KEY (input_at, sequence, section_sequence, term_collection_id, subject_code, course_number, school_id)
);
