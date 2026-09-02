# Using REP Without a Manifest

The `.rep.yaml` manifest is entirely optional. The gateway works with nothing
but environment variables — the `REP_PUBLIC_` / `REP_SENSITIVE_` /
`REP_SERVER_` prefix convention **is** the configuration.

The full walkthrough — renaming vars, installing the SDK, running the
gateway, changing config without a rebuild, and dev-mode file-watch hot
reload — lives on the docs site:

**[rep-protocol.dev/quick-start](https://rep-protocol.dev/quick-start/)**

When you outgrow prefix-only config, add a manifest for startup validation
and generated TypeScript types — see
[`examples/.rep.yaml`](./.rep.yaml) for a full annotated example, the
[todo-react example](./todo-react/README.md) for a complete app that uses
one, or the [manifest guide](https://rep-protocol.dev/guides/manifest/) on
the docs site.
