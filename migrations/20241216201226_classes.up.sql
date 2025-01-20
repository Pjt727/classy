-- can't use temp tables with sqlc so i use staging tables here
CREATE TYPE season_enum AS ENUM ('Spring', 'Fall', 'Winter', 'Summer');

CREATE TABLE schools (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL
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
    PRIMARY KEY (id, school_id)
);

CREATE TABLE previous_full_section_collections (
    school_id TEXT,
    collection_id TEXT,
    time_collection TIMESTAMP WITH TIME ZONE,

    FOREIGN KEY (collection_id, school_id) REFERENCES term_collections(id, school_id),
    PRIMARY KEY (collection_id, school_id, time_collection)
);

CREATE TABLE faculty_members (
    id TEXT,
    school_id TEXT,

    name TEXT NOT NULL,
    email_address TEXT,
    first_name TEXT,
    last_name TEXT,
    FOREIGN KEY (school_id) REFERENCES schools(id),
    PRIMARY KEY (id, school_id)
);

CREATE TABLE courses (
    id TEXT,
    school_id TEXT,

    subject_code TEXT,
    number TEXT,
    subject_description TEXT,
    title TEXT,
    description TEXT,
    credit_hours REAL NOT NULL,
    FOREIGN KEY (school_id) REFERENCES schools(id),
    PRIMARY KEY (id, school_id)
);

CREATE TABLE sections (
    id TEXT,
    term_collection_id TEXT,
    course_id TEXT,
    school_id TEXT,

    max_enrollment INTEGER,
    instruction_method TEXT,
    campus TEXT,
    enrollment INTEGER,
    primary_faculty_id TEXT,
    FOREIGN KEY (course_id, school_id) REFERENCES courses(id, school_id),
    FOREIGN KEY (primary_faculty_id, school_id) REFERENCES faculty_members(id, school_id),

    FOREIGN KEY (term_collection_id, school_id) REFERENCES term_collections(id, school_id),
    PRIMARY KEY (id, term_collection_id, course_id, school_id)
);

CREATE TABLE staging_sections (
    id TEXT NOT NULL,
    term_collection_id TEXT NOT NULL,
    course_id TEXT NOT NULL,
    school_id TEXT NOT NULL,

    max_enrollment INTEGER,
    instruction_method TEXT,
    campus TEXT,
    enrollment INTEGER,
    primary_faculty_id TEXT
);

CREATE TABLE meeting_times (
    sequence INT,
    section_id TEXT,
    term_collection_id TEXT,
    course_id TEXT,
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
    FOREIGN KEY (section_id, term_collection_id, course_id, school_id)
        REFERENCES sections(id, term_collection_id, course_id, school_id) ON DELETE CASCADE,
    PRIMARY KEY (sequence, section_id, term_collection_id, course_id, school_id)
);

CREATE TABLE staging_meeting_times (
    sequence INT NOT NULL,
    section_id TEXT NOT NULL,
    term_collection_id TEXT NOT NULL,
    course_id TEXT NOT NULL,
    school_id TEXT NOT NULL,

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
    is_sunday BOOLEAN NOT NULL
);
