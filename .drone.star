this_repo_slug = "tkushnir/avatarad"
program = "avatarad"
prg_version = "0.4.0"
registry = "docker.io"
username = "timophey73"

def main(ctx):
  if ctx.build.event == "tag" and ctx.build.branch == "master":
    name = "publish"
    dryrun = False
  else:
    name = "build image"
    dryrun = True
  p = [
    build(name, dryrun)
  ]
  return p

def build(name, dryrun):
  s = simple_stage("build %s" % program)
  trigger_repo(s, [ "cron", "custom", "push", "pull_request", "tag" ])
  ver = prg_version.split(".")
  bld_ver_option = "-X 'main.pkgVersion=%s.$DRONE_BUILD_NUMBER'" % prg_version
  add_volumes(s, mk_volume("certs", { "temp": {} }))
  add_volumes(s, mk_volume("ldif", { "temp": {} }))
  add_environment(s, {
    "LDAP_LOG_LEVEL": "0",
    "LDAP_ORGANISATION": "Example Org.",
    "LDAP_DOMAIN": "example.org",
    "LDAP_HOSTNAME": "ldapsrv",
    "LDAP_BIND_DN": "dc=example,dc=org",
    "LDAP_BIND_USER": "cn=admin,dc=example,dc=org",
    "LDAP_ADMIN_PASSWORD": "S3cr3t",
    "LDAP_TLS_CRT_FILENAME": "OpenLDAP_Server_cert.pem",
    "LDAP_TLS_KEY_FILENAME": "OpenLDAP_Server_key.pem",
    "LDAP_TLS_CA_CRT_FILENAME": "Certificate_Authority_cert.pem",
    "LDAP_TLS_VERIFY_CLIENT": "never"
  })
  add_steps(s, [
    {
      "name": "golang lint",
      "image": "golangci/golangci-lint:v2.0.2",
      "commands": [
        "golangci-lint run -v"
      ]
    },
    {
      "name": "copy ldap files",
      "image": "drone/git",
      "commands": [
        "cp -va certs/*.pem /tmp/certs",
        "cp -va ldif/*.ldif /tmp/ldif"
      ],
      "volumes": [
        { "name": "certs", "path": "/tmp/certs" },
        { "name": "ldif", "path": "/tmp/ldif" }
      ]
    },
    {
      "name": "ldapsrv",
      "image": "osixia/openldap",
      "detach": True,
      "command": [
        "--copy-service"
      ],
      "volumes": [
        { "name": "certs", "path": "/container/service/slapd/assets/certs" },
        { "name": "ldif", "path": "/container/service/slapd/assets/config/bootstrap/ldif/custom" }
      ]
    },
    {
      "name": "wait ldap online",
      "image": "mbentley/ldap-utils",
      "commands": [
        "export SSL_CERT_DIR=$DRONE_WORKSPACE/certs",
        "while ! ldapsearch -LLL -x -H ldaps://$LDAP_HOSTNAME -b $LDAP_BIND_DN -D $LDAP_BIND_USER -w $LDAP_ADMIN_PASSWORD objectClass=organization dn; do " +
          "sleep 5; " +
        "done"
      ],
      "environment": {
        "HOME": "/tmp"
      }
    },
    {
      "name": "test",
      "image": "golang:1.24.2",
      "commands": [
        "export GOPATH=$DRONE_WORKSPACE_BASE/go",
        "go mod download",
        "export SSL_CERT_FILE=certfile",
        "export SSL_CERT_DIR=/dev/null",
        "export LDAP_USER_BASE=ou=users,$LDAP_BIND_DN",
        'go test -test.v -coverprofile=coverage.out -ldflags "%s" -o %s.x ./%s/...' % (bld_ver_option, program, program)
      ]
    },
    {
      "name": "build program",
      "image": "golang:1.24.2-alpine3.21",
      "commands": [
        "export GOPATH=$DRONE_WORKSPACE_BASE/go",
        'go build -ldflags "-s -w %s" -o %s.x ./%s/...' % (bld_ver_option, program, program)
      ],
      "environment": {
        "GOOS": "linux",
        "GOARCH": "amd64",
        "GO111MODULE": "on"
      }
    },
    {
      "name": name,
      "image": "plugins/docker:20",
      "settings": {
        "tags": [
          "latest",
          prg_version,
          ver[0] + "." + ver[1],
          ver[0]
        ],
        "repo": "%s/%s/%s" % (registry, username, program),
        "username": username,
        "password": { "from_secret": "docker_password" },
        "dry_run": dryrun
      }
    }
  ])
  return s

def mk_volume(name, l = {}):
  v = {
    "name": name
  }
  v.update(l)
  return v

def add_volumes(p, l):
  add_node(p, l, "volumes")

def add_environment(p, t):
  if "environment" in p.keys():
    p["environment"].update(t)
  else:
    p.update({ "environment": t })

def simple_stage(name, wkspc_path = "${DRONE_REPO_NAME}"):
  s = {
    "kind": "pipeline",
    "type": "docker",
    "name": name
  }
  if wkspc_path != "":
    s.update({ "workspace": { "path": wkspc_path } })
  return s

def trigger_repo(s, t = {}):
  if type(t) != "dict":
    t = { "event": t }
  if "trigger" in s.keys():
    if "repo" in s["trigger"].keys():
      s["trigger"]["repo"] = this_repo_slug
    else:
      s["trigger"].update({ "repo": this_repo_slug })
  else:
    s.update({ "trigger": { "repo": this_repo_slug } })
  if t != {}:
    s["trigger"].update(t)

def add_steps(s, l):
  add_node(s, l, "steps")

def add_node(s, l, n):
  if type(l) != "list":
    l = [ l ]
  if n in s.keys():
    s[n] += l
  else:
    s.update({ n: l })
