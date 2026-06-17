#!/usr/bin/env python3
"""
Envoy configuration generator.

Reads:
  - profiles/<profile>/envoy.env → environment defaults for local or prod
  - shared/routes.yaml           → single source of truth for API gateway routing
  - envoy.tmpl.yaml  → Envoy config template with __PLACEHOLDER__ tokens

Generates:
  - envoy.yaml       → final Envoy configuration (DO NOT EDIT MANUALLY)

Usage:
  python3 generate_config.py                        # local profile
  ENVOY_PROFILE=prod python3 generate_config.py     # prod profile
  python3 generate_config.py --profile prod --dry-run
"""
import argparse
import os
import sys
import re
import yaml
import textwrap


# ──────────────────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────────────────

def indent(text: str, spaces: int) -> str:
    """Indent every line of *text* by *spaces* spaces."""
    return textwrap.indent(text, " " * spaces)


# ──────────────────────────────────────────────────────────
# Route generation
# ──────────────────────────────────────────────────────────

# The ext_authz disable block uses a YAML anchor (&) on its first occurrence
# and aliases (*) on subsequent ones. The anchor content is nested under
# typed_per_filter_config at +4/+6 spaces.
DISABLE_EXT_AUTHZ_ANCHOR = (
    "typed_per_filter_config: &disable_ext_authz\n"
    "    envoy.filters.http.ext_authz:\n"
    '      "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthzPerRoute\n'
    "      disabled: true"
)

DISABLE_EXT_AUTHZ_REF = "typed_per_filter_config: *disable_ext_authz"


def _make_ext_authz_disable(is_first_public: bool) -> str:
    """Return the ext_authz disable block — anchor on first use, alias afterwards."""
    if is_first_public:
        return DISABLE_EXT_AUTHZ_ANCHOR
    return DISABLE_EXT_AUTHZ_REF


def generate_envoy_routes(config: dict) -> str:
    """
    Generate the full `routes:` list for the Envoy virtual host.

    Processes routes in order from routes.yaml.  Each entry can be one of:
      - A redirect       (has `redirect_to`)
      - A cluster route  (has `cluster`)
      - A direct response (has `direct_response`)

    Returns raw YAML text (un-indented); the caller adds the 24-space indent.

    Indentation convention (relative to the `- ` list marker):
      - match:             (+0)
          path: ...        (+4, inside match)
          headers: ...     (+4, inside match)
        redirect:          (+2, sibling of match)
          path_redirect:   (+4)
        route:             (+2, sibling of match)
          cluster:         (+4)
        typed_per_filter_config:  (+2, sibling of match)
    """
    default_timeout = config.get("defaults", {}).get("timeout", "60s")
    entries = []
    first_public = True  # track YAML anchor placement

    for route in config.get("routes", []):
        is_public = route.get("public", False)
        parts = []

        # ── Match block ──
        match_path = route.get("path")
        match_prefix = route.get("prefix")
        method = route.get("method")

        match_lines = ["- match:"]
        if match_path:
            match_lines.append(f"    path: {match_path}")
        elif match_prefix:
            match_lines.append(f"    prefix: {match_prefix}")

        if method:
            match_lines.append("    headers:")
            match_lines.append('      - name: ":method"')
            match_lines.append("        string_match:")
            match_lines.append(f"          exact: {method}")

        parts.append("\n".join(match_lines))

        # ── Action block ──
        if "redirect_to" in route:
            parts.append(
                f"  redirect:\n"
                f"    path_redirect: {route['redirect_to']}\n"
                f"    response_code: MOVED_PERMANENTLY"
            )
        elif "direct_response" in route:
            dr = route["direct_response"]
            parts.append(
                f"  direct_response:\n"
                f"    status: {dr['status']}\n"
                f"    body:\n"
                f"      inline_string: '{dr['body']}'"
            )
        elif "cluster" in route:
            timeout = route.get("timeout", default_timeout)
            action = f"  route:\n    cluster: {route['cluster']}"
            if timeout is not None:
                action += f"\n    timeout: {timeout}"
            parts.append(action)

        # ── Ext authz disable ──
        if is_public:
            parts.append("  " + _make_ext_authz_disable(first_public))
            first_public = False

        entries.append("\n".join(parts))

    return "\n".join(entries)


def generate_jwt_bypass_rules(config: dict) -> str:
    """
    Generate the JWT `rules:` list from the unified routes.

    Public routes get a pass-through rule (no `requires`).
    The final catch-all rule `prefix: /` requires the keycloak provider.
    """
    rules = []

    for route in config.get("routes", []):
        if not route.get("public", False):
            continue
        # Skip direct_response entries — they're covered by the JWT catch-all
        if "direct_response" in route:
            continue

        match_path = route.get("path")
        match_prefix = route.get("prefix")
        if match_prefix:
            rules.append(f"- match:\n    prefix: {match_prefix}")
        elif match_path:
            rules.append(f"- match:\n    path: {match_path}")

    # Final catch-all: everything else requires JWT
    rules.append(
        "- match:\n"
        "    prefix: /\n"
        "  requires:\n"
        "    provider_name: keycloak"
    )

    return "\n".join(rules)


# ──────────────────────────────────────────────────────────
# Main pipeline
# ──────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Generate Envoy config from a selected environment profile.")
    parser.add_argument("--profile", default=os.environ.get("ENVOY_PROFILE", "local"), choices=["local", "prod"])
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--output", default=None)
    args = parser.parse_args()

    # Get the directory of this script to run relative paths reliably
    script_dir = os.path.dirname(os.path.abspath(__file__))

    env_path = os.path.join(script_dir, "profiles", args.profile, "envoy.env")
    template_path = os.path.join(script_dir, "envoy.tmpl.yaml")
    routes_path = os.path.join(script_dir, "shared", "routes.yaml")
    output_path = args.output or os.path.join(script_dir, "envoy.yaml")

    # ── 1. Load defaults from selected profile ──
    if not os.path.exists(env_path):
        print(f"Error: Environment file not found at {env_path}", file=sys.stderr)
        sys.exit(1)

    replacements = {
        "__ENVOY_GATEWAY_PROFILE__": args.profile,
    }
    with open(env_path, "r") as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if "=" not in line:
                print(f"Warning: Invalid format at {env_path}:{line_num}: {line}", file=sys.stderr)
                continue
            k, v = line.split("=", 1)
            key = k.strip()
            # Allow shell environment overrides, default to values in the selected profile.
            val = os.environ.get(key, v.strip())
            replacements[f"__{key}__"] = val

    # ── 2. Load routes definition ──
    if not os.path.exists(routes_path):
        print(f"Error: Routes file not found at {routes_path}", file=sys.stderr)
        sys.exit(1)

    with open(routes_path, "r") as f:
        routes_config = yaml.safe_load(f)

    # Generate route blocks
    routes_yaml = generate_envoy_routes(routes_config)
    jwt_rules_yaml = generate_jwt_bypass_rules(routes_config)

    # Add generated content as placeholders
    # These are indented to match their position in envoy.tmpl.yaml
    replacements["__ROUTES__"] = indent(routes_yaml, 24)
    replacements["__JWT_BYPASS_RULES__"] = indent(jwt_rules_yaml, 24)

    # ── 3. Read template configuration ──
    if not os.path.exists(template_path):
        print(f"Error: Template file not found at {template_path}", file=sys.stderr)
        sys.exit(1)

    with open(template_path, "r") as f:
        content = f.read()

    # ── 4. Perform substitutions ──
    for placeholder, val in replacements.items():
        content = content.replace(placeholder, val)

    # ── 5. Save generated configuration with warning header ──
    header = (
        "# WARNING: THIS FILE IS AUTOMATICALLY GENERATED BY generate_config.py.\n"
        "# DO NOT EDIT THIS FILE DIRECTLY. ANY CHANGES WILL BE OVERWRITTEN.\n"
        f"# Profile: {args.profile}\n"
        "# Edit envoy.tmpl.yaml, shared/routes.yaml, or profiles/<profile>/envoy.env instead.\n\n"
    )

    output = header + content

    if args.dry_run:
        print(output)
    else:
        with open(output_path, "w") as f:
            f.write(output)
        print(f"Generated {output_path} successfully.")

    # ── 6. Quality-of-life check: warn about any remaining unreplaced placeholders ──
    unreplaced = re.findall(r"__[A-Z0-9_]+__", content)
    if unreplaced:
        print(f"Warning: The following placeholders were not resolved: {set(unreplaced)}", file=sys.stderr)


if __name__ == "__main__":
    main()
