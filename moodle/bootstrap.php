<?php
// Bootstrap CloudPoolManager du Moodle local — exécuté DANS le conteneur via l'API Moodle.
// Idempotent : relançable sans dupliquer. Affiche le token WS sur stdout (ligne "TOKEN=...").
//   (appelé par scripts/moodle-bootstrap.sh)
define('CLI_SCRIPT', true);
require('/opt/bitnami/moodle/config.php');
require_once($CFG->dirroot.'/course/lib.php');
require_once($CFG->dirroot.'/user/lib.php');
require_once($CFG->dirroot.'/lib/externallib.php');
require_once($CFG->dirroot.'/lib/enrollib.php');
global $DB, $CFG;

// ── 1. Activer les Web Services + protocole REST ────────────────────────────
set_config('enablewebservices', 1);
// Service mobile : permet à n'importe quel utilisateur (élève) d'obtenir un token via
// login/token.php (validation des identifiants pour le "login via Moodle").
set_config('enablemobilewebservice', 1);
$protocols = (string)get_config('core', 'webserviceprotocols');
$list = array_filter(array_map('trim', explode(',', $protocols)));
if (!in_array('rest', $list)) { $list[] = 'rest'; }
set_config('webserviceprotocols', implode(',', $list));

// ── 2. Service externe dédié "cpm_service" + ses fonctions ──────────────────
$functions = [
    'core_webservice_get_site_info',
    'core_course_get_courses',
    'core_course_get_courses_by_field',
    'core_enrol_get_enrolled_users',
    'core_enrol_get_users_courses',
    'core_user_get_users',
    'core_user_get_users_by_field',
    'enrol_manual_enrol_users',
    'core_course_create_courses',
    'core_user_create_users',
    'mod_assign_get_assignments',
    'mod_assign_save_grade',
    'gradereport_user_get_grade_items',
];
$service = $DB->get_record('external_services', ['shortname' => 'cpm_service']);
if (!$service) {
    $service = (object)[
        'name' => 'CloudPoolManager', 'shortname' => 'cpm_service', 'enabled' => 1,
        'restrictedusers' => 0, 'downloadfiles' => 1, 'uploadfiles' => 1,
        'timecreated' => time(), 'timemodified' => time(),
    ];
    $service->id = $DB->insert_record('external_services', $service);
} else {
    $DB->set_field('external_services', 'enabled', 1, ['id' => $service->id]);
}
foreach ($functions as $fn) {
    $exists = $DB->record_exists('external_services_functions',
        ['externalserviceid' => $service->id, 'functionname' => $fn]);
    if (!$exists) {
        $DB->insert_record('external_services_functions',
            (object)['externalserviceid' => $service->id, 'functionname' => $fn]);
    }
}

// ── 3. Token permanent pour l'admin sur ce service ──────────────────────────
$admin = get_admin();
$context = context_system::instance();
$existing = $DB->get_record('external_tokens',
    ['externalserviceid' => $service->id, 'userid' => $admin->id, 'tokentype' => EXTERNAL_TOKEN_PERMANENT]);
if ($existing) {
    $token = $existing->token;
} else if (class_exists('\core_external\util') && method_exists('\core_external\util', 'generate_token')) {
    $token = \core_external\util::generate_token(
        EXTERNAL_TOKEN_PERMANENT, $service, $admin->id, $context, 0, '');
} else {
    $token = external_generate_token(
        EXTERNAL_TOKEN_PERMANENT, $service, $admin->id, $context, 0, '');
}

// ── 4. Cours de démo ────────────────────────────────────────────────────────
function ensure_course($shortname, $fullname) {
    global $DB;
    $c = $DB->get_record('course', ['shortname' => $shortname]);
    if ($c) { return $c; }
    $data = (object)[
        'category' => 1, 'fullname' => $fullname, 'shortname' => $shortname,
        'summary' => '', 'summaryformat' => FORMAT_HTML, 'format' => 'topics', 'visible' => 1,
    ];
    return create_course($data);
}
// Cours de démo : [shortname, fullname, nb d'élèves à inscrire].
$courseDefs = [
    ['CPM-PY101',  'Python 101 (démo CloudPoolManager)',                 12],
    ['CPM-DS200',  'Data Science 200 (démo CloudPoolManager)',           10],
    ['CPM-MEC431', 'Mécanique des fluides MEC431 (démo)',                 8],
    ['CPM-ECO589', 'Économie computationnelle ECO589 (démo)',            14],
    ['CPM-MAP579', 'Optimisation & calcul scientifique MAP579 (démo)',   16],
    ['CPM-BIO583', 'Bio-informatique BIO583 (démo)',                      6],
];
$courses = [];
$courseEnrolCount = [];
foreach ($courseDefs as $cd) {
    $c = ensure_course($cd[0], $cd[1]);
    $courses[] = $c;
    $courseEnrolCount[$c->id] = $cd[2];
}

// ── 5. Utilisateurs de démo ─────────────────────────────────────────────────
function ensure_user($username, $first, $last, $email) {
    global $DB, $CFG;
    $u = $DB->get_record('user', ['username' => $username]);
    if ($u) { return $u; }
    $user = (object)[
        'username' => $username, 'auth' => 'manual', 'confirmed' => 1,
        'mnethostid' => $CFG->mnet_localhost_id,
        'firstname' => $first, 'lastname' => $last, 'email' => $email,
        'password' => 'Student_2026!',
    ];
    $id = user_create_user($user, true, false);
    return $DB->get_record('user', ['id' => $id]);
}
// 18 élèves de démo (mot de passe commun Student_2026!).
$studentDefs = [
    ['alice',   'Alice',    'Martin',    'alice@example.com'],
    ['bob',     'Bob',      'Durand',    'bob@example.com'],
    ['charlie', 'Charlie',  'Bernard',   'charlie@example.com'],
    ['diana',   'Diana',    'Petit',     'diana@example.com'],
    ['emma',    'Emma',     'Robert',    'emma@example.com'],
    ['lucas',   'Lucas',    'Richard',   'lucas@example.com'],
    ['lea',     'Léa',      'Dubois',    'lea@example.com'],
    ['hugo',    'Hugo',     'Moreau',    'hugo@example.com'],
    ['chloe',   'Chloé',    'Laurent',   'chloe@example.com'],
    ['nathan',  'Nathan',   'Simon',     'nathan@example.com'],
    ['manon',   'Manon',    'Michel',    'manon@example.com'],
    ['enzo',    'Enzo',     'Lefebvre',  'enzo@example.com'],
    ['camille', 'Camille',  'Garcia',    'camille@example.com'],
    ['louis',   'Louis',    'David',     'louis@example.com'],
    ['sarah',   'Sarah',    'Bertrand',  'sarah@example.com'],
    ['theo',    'Théo',     'Roux',      'theo@example.com'],
    ['julie',   'Julie',    'Vincent',   'julie@example.com'],
    ['adam',    'Adam',     'Fournier',  'adam@example.com'],
];
$students = [];
foreach ($studentDefs as $sd) {
    $students[] = ensure_user($sd[0], $sd[1], $sd[2], $sd[3]);
}
$teacher = ensure_user('prof1', 'Paul', 'Prof', 'prof1@example.com');

// ── 6. Inscriptions (manual enrol) ──────────────────────────────────────────
function enrol_in($courseid, $userid, $roleshortname) {
    global $DB;
    $role = $DB->get_record('role', ['shortname' => $roleshortname]);
    if (!$role) { return; }
    $plugin = enrol_get_plugin('manual');
    $instance = $DB->get_record('enrol', ['courseid' => $courseid, 'enrol' => 'manual']);
    if (!$instance) {
        $course = $DB->get_record('course', ['id' => $courseid]);
        $plugin->add_default_instance($course);
        $instance = $DB->get_record('enrol', ['courseid' => $courseid, 'enrol' => 'manual']);
    }
    $plugin->enrol_user($instance, $userid, $role->id);
}
foreach ($courses as $i => $c) {
    // Effectif variable par cours, et décalage du point de départ pour varier la composition.
    $n = $courseEnrolCount[$c->id] ?? count($students);
    $offset = ($i * 3) % count($students);
    for ($k = 0; $k < $n && $k < count($students); $k++) {
        $s = $students[($offset + $k) % count($students)];
        enrol_in($c->id, $s->id, 'student');
    }
    enrol_in($c->id, $teacher->id, 'editingteacher');
    // L'admin (= utilisateur du token de service) doit être enseignant pour que
    // mod_assign_get_assignments liste les devoirs via les Web Services.
    enrol_in($c->id, $admin->id, 'editingteacher');
}

// ── 6b. Une activité "devoir" (mod_assign) par cours : cible du push de notes ──
require_once($CFG->dirroot.'/course/modlib.php');
function ensure_assign($courseid, $name) {
    global $DB, $CFG;
    $found = $DB->get_record('assign', ['course' => $courseid, 'name' => $name]);
    if ($found) { return $found->id; }
    $module = $DB->get_record('modules', ['name' => 'assign']);
    $mi = new stdClass();
    $mi->modulename = 'assign'; $mi->module = $module->id; $mi->course = $courseid;
    $mi->section = 0; $mi->visible = 1; $mi->name = $name; $mi->intro = ' '; $mi->introformat = FORMAT_HTML;
    $mi->grade = 100;
    $mi->submissiondrafts = 0; $mi->requiresubmissionstatement = 0;
    $mi->sendnotifications = 0; $mi->sendlatenotifications = 0; $mi->sendstudentnotifications = 1;
    $mi->duedate = 0; $mi->cutoffdate = 0; $mi->allowsubmissionsfromdate = 0; $mi->gradingduedate = 0;
    $mi->teamsubmission = 0; $mi->requireallteammemberssubmit = 0; $mi->teamsubmissiongroupingid = 0;
    $mi->blindmarking = 0; $mi->attemptreopenmethod = 'none'; $mi->maxattempts = -1;
    $mi->markingworkflow = 0; $mi->markingallocation = 0; $mi->completion = 0; $mi->completionsubmit = 0;
    $mi->assignsubmission_onlinetext_enabled = 0; $mi->assignsubmission_file_enabled = 0;
    $mi->assignsubmission_file_maxfiles = 1; $mi->assignsubmission_file_maxsizebytes = 0;
    $mi->assignfeedback_comments_enabled = 1; $mi->assignfeedback_file_enabled = 0; $mi->assignfeedback_offline_enabled = 0;
    $res = add_moduleinfo($mi, get_course($courseid));
    return $res->instance;
}
foreach ($courses as $c) {
    $aid = ensure_assign($c->id, 'TP nbgrader (démo)');
    echo "ASSIGN course={$c->id} assign_id={$aid}\n";
}

// ── 7. S'assurer que le service mobile est activé (login/token.php des élèves) ──
$DB->set_field('external_services', 'enabled', 1, ['shortname' => 'moodle_mobile_app']);
purge_all_caches();

// ── Résumé machine-lisible ──────────────────────────────────────────────────
echo "TOKEN=$token\n";
foreach ($courses as $c) { echo "COURSE id={$c->id} shortname={$c->shortname}\n"; }
echo "STUDENTS=" . implode(',', array_map(fn($s) => $s->email, $students)) . "\n";
echo "TEACHER=prof1@example.com (mot de passe Student_2026!)\n";
echo "OK\n";
