#!/usr/bin/env bash
# Read-only inventory of local BINTRANS Docker resources.
# Prints containers, volumes, and networks. Does not start, stop, remove, or prune.
set -euo pipefail

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not available" >&2
  exit 1
fi

classify() {
  local name="$1"
  local owner="$2"
  if [ -z "$owner" ] || [ "$owner" = "<no value>" ]; then
    owner=""
  fi
  case "$name" in
    freight_*)
      if [ -n "$owner" ]; then
        echo OWNER_MISMATCH
      else
        echo CANONICAL
      fi
      ;;
    bintrans-a-*|bintrans_a_*)
      if [ -z "$owner" ]; then echo UNLABELED
      elif [ "$owner" = "agent-a" ]; then echo AGENT_A
      else echo OWNER_MISMATCH
      fi
      ;;
    bintrans-b-*|bintrans_b_*)
      if [ -z "$owner" ]; then echo UNLABELED
      elif [ "$owner" = "agent-b" ]; then echo AGENT_B
      else echo OWNER_MISMATCH
      fi
      ;;
    bintrans-c-*|bintrans_c_*)
      if [ -z "$owner" ]; then echo UNLABELED
      elif [ "$owner" = "agent-c" ]; then echo AGENT_C
      else echo OWNER_MISMATCH
      fi
      ;;
    bintrans-d-*|bintrans_d_*)
      if [ -z "$owner" ]; then echo UNLABELED
      elif [ "$owner" = "agent-d" ]; then echo AGENT_D
      else echo OWNER_MISMATCH
      fi
      ;;
    bintrans*)
      echo UNLABELED
      ;;
    *)
      echo UNKNOWN
      ;;
  esac
}

echo "CONTAINER|PROJECT|WORKDIR|SERVICE|OWNER|TASK|LIFECYCLE|GIT_SHA|STATUS|CREATED|RESTART|CHECK"
while IFS= read -r id; do
  [ -z "$id" ] && continue
  line="$(docker inspect "$id" --format '{{.Name}}|{{index .Config.Labels "com.docker.compose.project"}}|{{index .Config.Labels "com.docker.compose.project.working_dir"}}|{{index .Config.Labels "com.docker.compose.service"}}|{{index .Config.Labels "com.bintrans.owner"}}|{{index .Config.Labels "com.bintrans.task"}}|{{index .Config.Labels "com.bintrans.lifecycle"}}|{{index .Config.Labels "com.bintrans.git-sha"}}|{{.State.Status}}|{{.Created}}|{{.HostConfig.RestartPolicy.Name}}')"
  name="${line%%|*}"
  name="${name#/}"
  owner_field="$(printf '%s' "$line" | awk -F'|' '{print $5}')"
  check="$(classify "$name" "$owner_field")"
  printf '%s|%s\n' "$line" "$check" | sed 's#^/##'
done < <(docker ps -aq)

echo "VOLUME|NAME|CREATED|OWNER|TASK|LIFECYCLE|GIT_SHA|COMPOSE_PROJECT"
while IFS= read -r vol; do
  [ -z "$vol" ] && continue
  docker volume inspect "$vol" --format '{{.Name}}|{{.CreatedAt}}|{{index .Labels "com.bintrans.owner"}}|{{index .Labels "com.bintrans.task"}}|{{index .Labels "com.bintrans.lifecycle"}}|{{index .Labels "com.bintrans.git-sha"}}|{{index .Labels "com.docker.compose.project"}}' \
    | awk -F'|' 'BEGIN{OFS="|"} {for(i=1;i<=NF;i++) if($i=="" || $i=="<no value>") $i="-"; print "VOLUME",$0}'
done < <(docker volume ls -q)

echo "NETWORK|NAME|OWNER|TASK|LIFECYCLE|GIT_SHA|COMPOSE_PROJECT"
while IFS= read -r net; do
  [ -z "$net" ] && continue
  case "$net" in
    bridge|host|none) continue ;;
  esac
  docker network inspect "$net" --format '{{.Name}}|{{index .Labels "com.bintrans.owner"}}|{{index .Labels "com.bintrans.task"}}|{{index .Labels "com.bintrans.lifecycle"}}|{{index .Labels "com.bintrans.git-sha"}}|{{index .Labels "com.docker.compose.project"}}' \
    | awk -F'|' 'BEGIN{OFS="|"} {for(i=1;i<=NF;i++) if($i=="" || $i=="<no value>") $i="-"; print "NETWORK",$0}'
done < <(docker network ls --format '{{.Name}}')
