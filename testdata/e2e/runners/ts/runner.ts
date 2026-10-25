// Runner of the e2e scenarios for typescript-fetch clients: calls their
// operations as the scenarios say and prints one JSON line per case (see
// testdata/e2e/README.md). main.ts, written for each run, hands it the
// clients and the directories of their sources. Node's own fetch, FormData
// and File carry the requests. Types come from the clients' sources: the
// <Op>Request interfaces and the method signatures say which model's
// FromJSON/ToJSON turns wire JSON into the client's objects and back.

declare const require: any;
declare const process: any;
declare const Buffer: any;

const fs = require("fs");
const path = require("path");

type Auth = { username?: string; password?: string; apiKey?: string; bearer?: string };

type Case = { op: string; params: string[]; args?: Record<string, any>; auth?: Auth; save?: Record<string, string> };

type Suite = { name: string; base: string; mock: boolean; auth?: Auth; cases: Case[]; schemes?: Record<string, string> };

type Client = { module: any; dir: string };

type Method = { name: string; api: string; request?: string; result: string };

const norm = (s: string): string => s.replace(/[^\p{L}\p{N}]/gu, "").toLowerCase();

// methods reads the typed method of each operation from the API sources:
// its name, the API class, the Request interface it takes and what it
// returns.
function methods(dir: string): { methods: Method[]; requests: Record<string, Record<string, string>> } {
    const out: Method[] = [];
    const requests: Record<string, Record<string, string>> = {};

    for (const file of fs.readdirSync(path.join(dir, "apis"))) {
        const src: string = fs.readFileSync(path.join(dir, "apis", file), "utf8");
        const api = /export class (\w+) extends runtime\.BaseAPI/.exec(src)?.[1];

        for (const m of src.matchAll(/export interface (\w+) \{([\s\S]*?)\n\}/g)) {
            const fields: Record<string, string> = {};
            for (const f of m[2].matchAll(/^\s+(\w+)\??: (.+);$/gm)) {
                fields[f[1]] = f[2];
            }

            requests[m[1]] = fields;
        }

        if (!api) {
            continue;
        }

        for (const m of src.matchAll(/^    async (\w+)\((?:requestParameters: (\w+)(?: = \{\})?, )?initOverrides\?: [^)]*\): Promise<(.*)> \{$/gm)) {
            if (!m[1].endsWith("Raw")) {
                out.push({ name: m[1], api, request: m[2], result: m[3] });
            }
        }
    }

    return { methods: out, requests };
}

const nullable = (t: string): string => t.replace(/ \| null$/, "").replace(/ \| undefined$/, "");

// fromWire turns a wire value into what the client takes for type t.
function fromWire(client: any, t: string, v: any): any {
    t = nullable(t);
    if (v === null || v === undefined) {
        return v;
    }

    if (v && typeof v === "object" && (v.$file || v.$bytes)) {
        const c = v.$file ?? v.$bytes;
        const data = c.base64 !== undefined ? Buffer.from(c.base64, "base64") : Buffer.from(c.text ?? "", "utf8");

        return v.$file ? new File([data], c.name) : new Blob([data]);
    }

    let m: RegExpExecArray | null;
    if ((m = /^Array<(.+)>$/.exec(t)) && Array.isArray(v)) {
        return v.map((x) => fromWire(client, m![1], x));
    }

    if ((m = /^\{ \[key: string\]: (.+); \}$/.exec(t)) && typeof v === "object") {
        return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, fromWire(client, m![1], x)]));
    }

    if (t === "Date") {
        return new Date(v);
    }

    const from = client[t + "FromJSON"];

    return typeof from === "function" ? from(v) : v;
}

// toWire turns a client's result of type t back into wire JSON.
async function toWire(client: any, t: string, v: any): Promise<any> {
    t = nullable(t);
    if (v === null || v === undefined) {
        return v ?? null;
    }

    if (v instanceof Blob) {
        return Buffer.from(await v.arrayBuffer()).toString("base64");
    }

    let m: RegExpExecArray | null;
    if ((m = /^Array<(.+)>$/.exec(t)) && Array.isArray(v)) {
        return Promise.all(v.map((x) => toWire(client, m![1], x)));
    }

    if ((m = /^\{ \[key: string\]: (.+); \}$/.exec(t)) && typeof v === "object") {
        const out: Record<string, any> = {};
        for (const [k, x] of Object.entries(v)) {
            out[k] = await toWire(client, m[1], x);
        }

        return out;
    }

    if (v instanceof Date) {
        return v.toISOString();
    }

    const to = client[t + "ToJSON"];

    return typeof to === "function" ? to(v) : v;
}

// expand replaces ${name} with a value a case before saved, escaped for a
// JSON string when json; a name never saved is an error.
function expand(s: string, vars: Record<string, string>, json: boolean): string {
    return s.replace(/\$\{(\w+)\}/g, (ref, name) => {
        if (!(name in vars)) {
            throw new Error(ref + " was never saved");
        }

        return json ? JSON.stringify(vars[name]).slice(1, -1) : vars[name];
    });
}

function save(result: any, paths: Record<string, string> | undefined, vars: Record<string, string>) {
    for (const [name, p] of Object.entries(paths ?? {})) {
        let v = result;
        for (const key of p.split(".").filter(Boolean)) {
            v = v?.[Array.isArray(v) ? Number(key) : key];
        }

        vars[name] = typeof v === "string" ? v : JSON.stringify(v);
    }
}

async function callCase(client: Client, info: ReturnType<typeof methods>, suite: Suite, c: Case, auth: Auth, vars: Record<string, string>) {
    const cfg = new client.module.Configuration({
        basePath: suite.base,
        username: auth.username,
        password: auth.password,
        apiKey: () => auth.apiKey ?? "",
        // typescript-fetch sends an OAuth2 token as the whole Authorization
        // header and prefixes only an HTTP bearer one: the function returns
        // "Bearer ..." for OAuth2, as its users write it.
        accessToken: (name?: string) => {
            const oauth = ["oauth2", "openIdConnect"].includes(suite.schemes?.[name ?? ""] ?? "");

            return auth.bearer ? (oauth ? "Bearer " : "") + auth.bearer : "";
        },
    });

    const want = [norm(c.op) + "operation", norm(c.op) + "method", norm(c.op)];
    const m = want.map((w) => info.methods.find((x) => norm(x.name) === w)).find(Boolean);
    if (!m) {
        return { fail: "no method for operation " + c.op };
    }

    const api = new client.module[m.api](cfg);
    const args = c.args ? JSON.parse(expand(JSON.stringify(c.args), vars, true)) : {};

    const fields = m.request ? (info.requests[m.request] ?? {}) : {};
    const byNorm: Record<string, string> = {};
    // The second parameter of one identifier is numbered, as generators do.
    for (const p of c.params ?? []) {
        let key = norm(p);
        for (let n = 2; key in byNorm; n++) {
            key = norm(p) + n;
        }

        byNorm[key] = p;
    }

    const params: Record<string, any> = {};
    const used = new Set<string>();

    for (const [field, type] of Object.entries(fields)) {
        const name = byNorm[norm(field)] ?? "body";
        if (!(name in args)) {
            continue;
        }

        if (used.has(name)) {
            return { fail: `arg ${name}: fits fields ${field} and another` };
        }

        used.add(name);
        params[field] = fromWire(client.module, type, args[name]);
    }

    for (const name of Object.keys(args)) {
        if (!used.has(name)) {
            return { fail: `arg ${name}: no field of ${m.request ?? m.name} takes it` };
        }
    }

    try {
        const result = m.request ? await api[m.name](params) : await api[m.name]();
        const wire = m.result === "void" ? null : await toWire(client.module, m.result, result);
        save(wire, c.save, vars);

        return { result: wire ?? null };
    } catch (e: any) {
        if (e?.response && typeof e.response.status === "number") {
            const body = await e.response.text();
            let model: any;
            try {
                model = JSON.parse(body);
            } catch {
                model = undefined;
            }

            return { error: { status: e.response.status, model, body } };
        }

        return { fail: String(e?.cause ?? e) };
    }
}

export async function run(clients: Record<string, Client>) {
    const input = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));

    for (const suite of input.suites as Suite[]) {
        const client = clients[suite.name];
        const info = methods(client.dir);
        const vars: Record<string, string> = {};
        let auth: Auth = { ...(suite.auth ?? {}) };

        for (const [i, c] of suite.cases.entries()) {
            for (const [k, v] of Object.entries(c.auth ?? {})) {
                if (v) {
                    (auth as any)[k] = v;
                }
            }

            const current: Auth = {};
            for (const [k, v] of Object.entries(auth)) {
                try {
                    (current as any)[k] = expand(v ?? "", vars, false);
                } catch {
                    (current as any)[k] = v;
                }
            }

            if (suite.mock) {
                await fetch(`${suite.base}/__e2e/case/${i}`, { method: "POST" });
            }

            let out: any;
            try {
                out = await callCase(client, info, suite, c, current, vars);
            } catch (e: any) {
                out = { fail: "panic: " + String(e?.stack ?? e) };
            }

            process.stdout.write(JSON.stringify({ suite: suite.name, case: i, ...out }) + "\n");
        }
    }
}
