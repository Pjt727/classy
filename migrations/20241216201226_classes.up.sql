DROP TYPE IF EXISTS season_enum;
CREATE TYPE season_enum AS ENUM ('Spring', 'Fall', 'Winter', 'Summer');

CREATE TABLE schools (
    id TEXT PRIMARY KEY,
    name TEXT
);

CREATE TABLE terms (
    year INT,
    season season_enum,

    PRIMARY KEY (year, season)
);


CREATE TABLE faculty_members (
    id TEXT,
    school_id TEXT,

    name TEXT NOT NULL,
    email_address TEXT,
    first_name TEXT,
    last_name TEXT,
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
    credit_hours INTEGER NOT NULL,
    FOREIGN KEY (school_id) REFERENCES schools(id),
    PRIMARY KEY (id, school_id)
);

CREATE TABLE sections (
    id TEXT,
    term_season season_enum,
    term_year INT,
    course_id TEXT,
    school_id TEXT,

    max_enrollment INTEGER,
    instruction_method TEXT,
    campus TEXT,
    enrollment INTEGER,
    primary_faculty_id TEXT,
    FOREIGN KEY (course_id, school_id) REFERENCES courses(id, school_id),
    FOREIGN KEY (primary_faculty_id, school_id) REFERENCES faculty_members(id, school_id),
    FOREIGN KEY (term_year, term_season) REFERENCES terms(year, season),
    PRIMARY KEY (id, term_season, term_year, course_id, school_id)
);

CREATE TABLE meeting_times (
    id SERIAL,
    section_id TEXT,
    term_season season_enum,
    term_year INT,
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
    FOREIGN KEY (section_id, term_season, term_year, course_id, school_id) REFERENCES sections(id, term_season, term_year, course_id, school_id),
    PRIMARY KEY (id, section_id, term_season, term_year, course_id, school_id)
);
