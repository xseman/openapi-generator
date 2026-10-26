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

type Case = { op: string; params: string[]; bodies?: string[]; args?: Record<string, any>; auth?: Auth; save?: Record<string, string> };

type Suite = { name: string; base: string; mock: boolean; auth?: Auth; cases: Case[]; schemes?: Record<string, string> };

type Client = { load: () => any; dir: string; module?: any };

// A Method's day says that its result's dates are days (format: date),
// which a Date holds as local midnight.
type Method = { name: string; api: string; request?: string; result: string; day: boolean };

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
                const raw = src.slice(src.indexOf(`    async ${m[1]}Raw(`)).split("\n    }\n")[0];
                out.push({ name: m[1], api, request: m[2], result: m[3], day: /runtime\.parseDate\b/.test(raw) });
            }
        }
    }

    return { methods: out, requests };
}

// Decl is a model of a client: an interface's properties (with those of
// the interface it extends, and whether an index signature takes others),
// their JSON names and the read-only ones, or a type alias.
type Decl = { fields?: Record<string, string>; base?: string; open?: boolean; alias?: string; wire?: Record<string, string>; readonly?: Set<string> };

// models reads the models' interfaces and type aliases from the sources.
function models(dir: string): Record<string, Decl> {
    const out: Record<string, Decl> = {};
    const d = path.join(dir, "models");

    for (const file of fs.existsSync(d) ? fs.readdirSync(d) : []) {
        const src: string = fs.readFileSync(path.join(d, file), "utf8");

        // A FromJSON reads each property's JSON name: "name": ... json["wire"].
        const wire: Record<string, string> = {};
        for (const f of src.matchAll(/^        "([^"]+)": .*?json\["([^"]+)"\]/gm)) {
            wire[f[1]] = f[2];
        }

        for (const m of src.matchAll(/^export interface (\w+)(?: extends (\w+))? \{([\s\S]*?)\n\}/gm)) {
            const fields: Record<string, string> = {};
            const readonly = new Set<string>();
            for (const f of m[3].matchAll(/^    (readonly )?("[^"]+"|[\w$]+)\??: (.+);$/gm)) {
                const name = f[2].replace(/^"|"$/g, "");
                fields[name] = f[3];
                if (f[1]) {
                    readonly.add(name);
                }
            }

            // An interface without properties, as TypeScript's {}, takes any object.
            const open = /^    \[key: string\]/m.test(m[3]) || (Object.keys(fields).length === 0 && !m[2]);
            out[m[1]] = { fields, base: m[2], open, wire, readonly };
        }

        for (const m of src.matchAll(/^export type (\w+) = (.+);$/gm)) {
            out[m[1]] ??= { alias: m[2] };
        }
    }

    return out;
}

// members splits a type at its top-level |.
function members(t: string): string[] {
    const out: string[] = [];
    let depth = 0;
    let start = 0;

    for (let i = 0; i < t.length; i++) {
        if ("<{(".includes(t[i])) {
            depth++;
        } else if (">})".includes(t[i])) {
            depth--;
        } else if (t[i] === "|" && depth === 0) {
            out.push(t.slice(start, i).trim());
            start = i + 1;
        }
    }

    return [...out, t.slice(start).trim()];
}

const show = (v: any): string => (v instanceof Date ? "Date " + v.toISOString() : (JSON.stringify(v) ?? String(v)).slice(0, 80));

// shape says where v is not of the client's type t, or "": an object of a
// model with a key its interface lacks (a wire name FromJSON left), a
// string where a Date belongs. A FromJSON and a ToJSON that both leave a
// value alone pass a round trip; they do not pass this. Scalars count in
// results only: the runner hands arguments over as the scenario has them.
function shape(decls: Record<string, Decl>, t: string, v: any, at: string, scalars: boolean): string {
    if (v === null || v === undefined) {
        return "";
    }

    const alts = members(t).filter((x) => x !== "null" && x !== "undefined");
    if (alts.length > 1) {
        const errs = alts.map((x) => shape(decls, x, v, at, scalars));

        return errs.every(Boolean) ? errs[0] : "";
    }

    t = alts[0] ?? "any";

    let m: RegExpExecArray | null;
    if ((m = /^(Array|Set)<(.+)>$/.exec(t))) {
        if (!(m[1] === "Set" ? v instanceof Set : Array.isArray(v))) {
            return `${at}: want ${t}, got ${show(v)}`;
        }

        let i = 0;
        for (const x of v) {
            const e = shape(decls, m[2], x, `${at}[${i++}]`, scalars);
            if (e) {
                return e;
            }
        }

        return "";
    }

    if ((m = /^\{ \[key: string\]: (.+); \}$/.exec(t) ?? /^Record<string, (.+)>$/.exec(t))) {
        if (typeof v !== "object" || Array.isArray(v)) {
            return `${at}: want a map, got ${show(v)}`;
        }

        for (const [k, x] of Object.entries(v)) {
            const e = shape(decls, m[1], x, `${at}.${k}`, scalars);
            if (e) {
                return e;
            }
        }

        return "";
    }

    if (t === "Date") {
        return v instanceof Date && !isNaN(v.getTime()) ? "" : `${at}: want a Date, got ${show(v)}`;
    }

    if (t === "Blob") {
        return v instanceof Blob ? "" : `${at}: want a Blob, got ${show(v)}`;
    }

    if (["string", "number", "boolean"].includes(t)) {
        return !scalars || typeof v === t ? "" : `${at}: want a ${t}, got ${show(v)}`;
    }

    const d = decls[t];
    if (!d) {
        return "";
    }

    if (d.alias !== undefined) {
        return d.alias.startsWith("typeof ") ? "" : shape(decls, d.alias, v, at, scalars);
    }

    // An interface without properties is TypeScript's {}: any value but null.
    if (!d.base && Object.keys(d.fields ?? {}).length === 0) {
        return "";
    }

    if (typeof v !== "object" || Array.isArray(v) || v instanceof Date || v instanceof Blob) {
        return `${at}: want a ${t}, got ${show(v)}`;
    }

    // A model that extends t (a child a discriminator picked) is a t too.
    const own = fits(decls, t, v, at, scalars);
    const isChild = (n: string): boolean => {
        for (let c = decls[n]; c?.base; c = decls[c.base]) {
            if (c.base === t) {
                return true;
            }
        }

        return false;
    };

    return own && Object.keys(decls).some((n) => isChild(n) && !fits(decls, n, v, at, scalars)) ? "" : own;
}

// fits says where v is not an object of model t, or "".
function fits(decls: Record<string, Decl>, t: string, v: any, at: string, scalars: boolean): string {
    const d = decls[t];
    const fields: Record<string, string> = {};
    let open = false;
    for (let c: Decl | undefined = d; c; c = c.base ? decls[c.base] : undefined) {
        Object.assign(fields, { ...c.fields, ...fields });
        open ||= !!c.open;
    }

    for (const [k, x] of Object.entries(v)) {
        if (!(k in fields)) {
            if (open) {
                continue;
            }

            return `${at}: ${k} is no property of ${t}`;
        }

        const e = shape(decls, fields[k], x, `${at}.${k}`, scalars);
        if (e) {
            return e;
        }
    }

    return "";
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

// toWire turns a client's result of type t back into wire JSON; day says
// that a Date is a day.
async function toWire(client: any, decls: Record<string, Decl>, t: string, v: any, day = false): Promise<any> {
    t = nullable(t);
    if (v === null || v === undefined) {
        return v ?? null;
    }

    if (v instanceof Blob) {
        return Buffer.from(await v.arrayBuffer()).toString("base64");
    }

    let m: RegExpExecArray | null;
    if ((m = /^Array<(.+)>$/.exec(t)) && Array.isArray(v)) {
        return Promise.all(v.map((x) => toWire(client, decls, m![1], x, day)));
    }

    // A stream is the list of its items, an event { data, event, id, retry }.
    if ((m = /^AsyncIterable<(.+)>$/.exec(t))) {
        const items: any[] = [];
        for await (const x of v) {
            items.push(await toWire(client, decls, m[1], x, day));
        }

        return items;
    }

    if ((m = /^runtime\.ServerSentEvent<(.+)>$/.exec(t))) {
        return { ...v, data: await toWire(client, decls, m[1], v.data, day) };
    }

    if ((m = /^\{ \[key: string\]: (.+); \}$/.exec(t)) && typeof v === "object") {
        const out: Record<string, any> = {};
        for (const [k, x] of Object.entries(v)) {
            out[k] = await toWire(client, decls, m[1], x, day);
        }

        return out;
    }

    if (v instanceof Date) {
        return day ? client.serializeDate(v) : v.toISOString();
    }

    const to = client[t + "ToJSON"];
    if (typeof to !== "function") {
        return v;
    }

    const out = to(v);
    await restore(client, decls, t, v, out);

    return out;
}

// restore puts back the read-only properties a model's ToJSON leaves out,
// as deep as the value goes: a result holds them, a request does not.
async function restore(client: any, decls: Record<string, Decl>, t: string, v: any, out: any): Promise<void> {
    t = nullable(t);
    if (v == null || out == null || typeof out !== "object") {
        return;
    }

    let m: RegExpExecArray | null;
    if ((m = /^Array<(.+)>$/.exec(t)) || (m = /^\{ \[key: string\]: (.+); \}$/.exec(t))) {
        for (const k of Object.keys(v)) {
            await restore(client, decls, m[1], v[k as any], out[k as any]);
        }

        return;
    }

    for (let d: Decl | undefined = decls[t]; d; d = d.base ? decls[d.base] : undefined) {
        for (const [f, type] of Object.entries(d.fields ?? {})) {
            const w = d.wire?.[f];
            if (w === undefined || v[f] === undefined) {
                continue;
            }

            if (d.readonly?.has(f)) {
                out[w] = await toWire(client, decls, type, v[f]);
            } else {
                await restore(client, decls, type, v[f], out[w]);
            }
        }
    }
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

async function callCase(client: Client, info: ReturnType<typeof methods>, decls: Record<string, Decl>, suite: Suite, c: Case, auth: Auth, vars: Record<string, string>) {
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

        const bad = shape(decls, type, params[field], "arg " + name, false);
        if (bad) {
            return { fail: "FromJSON: " + bad };
        }
    }

    for (const name of Object.keys(args)) {
        if (!used.has(name)) {
            return { fail: `arg ${name}: no field of ${m.request ?? m.name} takes it` };
        }
    }

    try {
        const result = m.request ? await api[m.name](params) : await api[m.name]();

        const bad = shape(decls, m.result, result, "result", true);
        if (bad) {
            return { fail: "the result is not of its type: " + bad };
        }

        const wire = m.result === "void" ? null : await toWire(client.module, decls, m.result, result, m.day);
        save(wire, c.save, vars);

        return { result: wire ?? null };
    } catch (e: any) {
        if (e?.response && typeof e.response.status === "number") {
            const body = await e.response.text();
            const status = String(e.response.status);
            // The client has no error model: the body parsed as JSON stands
            // for one where the spec declares a body for the status.
            let model: any;
            try {
                if (c.bodies?.some((k) => k === status || k === status[0] + "XX" || k === "default")) {
                    model = JSON.parse(body);
                }
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
        try {
            client.module = client.load();
        } catch (e: any) {
            for (const i of suite.cases.keys()) {
                process.stdout.write(JSON.stringify({ suite: suite.name, case: i, fail: "the client does not load: " + String(e) }) + "\n");
            }

            continue;
        }

        const info = methods(client.dir);
        const decls = models(client.dir);
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
                out = await callCase(client, info, decls, suite, c, current, vars);
            } catch (e: any) {
                out = { fail: "panic: " + String(e?.stack ?? e) };
            }

            process.stdout.write(JSON.stringify({ suite: suite.name, case: i, ...out }) + "\n");
        }
    }
}
