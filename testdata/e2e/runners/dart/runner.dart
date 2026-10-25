// Runner of the e2e scenarios for dart-fetch clients: calls their
// operations as the scenarios say and prints one JSON line per case (see
// testdata/e2e/README.md). main.dart, written for each run, hands it a
// factory of each client's Configuration and APIs; dart:mirrors finds the
// methods, their named parameters and the models' fromJson/toJson, so it
// runs on the VM (`dart run`), not compiled ahead of time.
import 'dart:convert';
import 'dart:io';
import 'dart:mirrors';

class Auth {
  String? username, password, apiKey, bearer;

  Auth.fromJson(Map<String, dynamic>? j)
      : username = j?['username'],
        password = j?['password'],
        apiKey = j?['apiKey'],
        bearer = j?['bearer'];

  void merge(Auth o) {
    username = o.username?.isNotEmpty == true ? o.username : username;
    password = o.password?.isNotEmpty == true ? o.password : password;
    apiKey = o.apiKey?.isNotEmpty == true ? o.apiKey : apiKey;
    bearer = o.bearer?.isNotEmpty == true ? o.bearer : bearer;
  }
}

/// Client is one generated package: its Configuration for a base URL and
/// credentials, and its API objects for a Configuration.
class Client {
  Client(this.configuration, this.apis, this.package);

  final Object Function(String base, Auth auth) configuration;
  final List<Object> Function(Object cfg) apis;
  final String package;
}

String norm(String s) => s.replaceAll(RegExp(r'[^\p{L}\p{N}]', unicode: true), '').toLowerCase();

String expand(String s, Map<String, String> vars, bool json) =>
    s.replaceAllMapped(RegExp(r'\$\{(\w+)\}'), (m) {
      final v = vars[m[1]];
      if (v == null) throw StateError('${m[0]} was never saved');
      if (!json) return v;
      final e = jsonEncode(v);
      return e.substring(1, e.length - 1);
    });

/// fromWire turns a wire value into what a parameter of type t takes.
Object? fromWire(TypeMirror t, Object? v) {
  if (v == null) return null;

  if (v is Map && (v.containsKey(r'$file') || v.containsKey(r'$bytes'))) {
    final c = (v[r'$file'] ?? v[r'$bytes']) as Map;
    return c['base64'] != null ? base64Decode(c['base64'] as String) : utf8.encode((c['text'] ?? '') as String);
  }

  if (t is ClassMirror && t.typeArguments.isNotEmpty) {
    if (t.isSubtypeOf(reflectType(List)) && v is List) {
      final item = t.typeArguments.first;
      return _typedList(item, v.map((x) => fromWire(item, x)).toList());
    }

    if (t.isSubtypeOf(reflectType(Map)) && v is Map) {
      final value = t.typeArguments.last;
      return _typedMap(value, {for (final e in v.entries) e.key as String: fromWire(value, e.value)});
    }
  }

  if (t is ClassMirror) {
    if (t.reflectedType == DateTime && v is String) return DateTime.parse(v);
    if (t.reflectedType == double && v is num) return v.toDouble();
    // A model reads JSON with its fromJson constructor, an enum with a
    // static fromJson.
    if (t.declarations.keys.any((k) => MirrorSystem.getName(k) == '${MirrorSystem.getName(t.simpleName)}.fromJson')) {
      return t.newInstance(#fromJson, [v]).reflectee;
    }

    if (t.staticMembers.containsKey(#fromJson)) return t.invoke(#fromJson, [v]).reflectee;
  }

  return v;
}

/// _typedList and _typedMap build a List<T> or Map<String, T> of the
/// parameter's own T, which a plain List<Object?> would not satisfy.
Object _typedList(TypeMirror item, List<Object?> values) {
  final list = (reflectType(List, [item.reflectedType]) as ClassMirror).newInstance(#empty, [], {#growable: true}).reflectee as List;
  for (final x in values) {
    list.add(x);
  }
  return list;
}

Object _typedMap(TypeMirror value, Map<String, Object?> values) {
  final map = (reflectType(Map, [String, value.reflectedType]) as ClassMirror).newInstance(Symbol(''), []).reflectee as Map;
  values.forEach((k, x) => map[k] = x);
  return map;
}

/// toWire turns a result back into wire JSON.
Object? toWire(Object? v) {
  if (v == null || v is String || v is num || v is bool) return v;
  if (v is DateTime) return v.toUtc().toIso8601String();
  if (v is List<int> && v is! List<num>) return base64Encode(v);
  if (v is List) return v.map(toWire).toList();
  if (v is Map) return {for (final e in v.entries) '${e.key}': toWire(e.value)};

  final m = reflect(v);
  if (m.type.instanceMembers.containsKey(#toJson)) return toWire(m.invoke(#toJson, []).reflectee);

  return v.toString();
}

void save(Object? result, Map<String, dynamic>? paths, Map<String, String> vars) {
  paths?.forEach((name, p) {
    Object? v = result;
    for (final key in (p as String).split('.').where((k) => k.isNotEmpty)) {
      v = v is List ? v[int.parse(key)] : (v is Map ? v[key] : null);
    }
    vars[name] = v is String ? v : jsonEncode(v);
  });
}

Future<Map<String, Object?>> callCase(Client client, Map<String, dynamic> suite, Map<String, dynamic> c, Auth auth,
    Map<String, String> vars) async {
  final cfg = client.configuration(suite['base'] as String, auth);
  final op = norm(c['op'] as String);

  for (final api in client.apis(cfg)) {
    final m = reflect(api);
    final method = m.type.instanceMembers.values.where((x) => x.isRegularMethod && norm(MirrorSystem.getName(x.simpleName)) == op);
    if (method.isEmpty) continue;

    final mm = method.first;
    final args = c['args'] == null ? <String, dynamic>{} : jsonDecode(expand(jsonEncode(c['args']), vars, true)) as Map<String, dynamic>;
    final byNorm = {for (final p in (c['params'] as List? ?? [])) norm(p as String): p};
    final named = <Symbol, Object?>{};
    final used = <String>{};

    for (final p in mm.parameters) {
      final field = MirrorSystem.getName(p.simpleName);
      final name = byNorm[norm(field)] ?? 'body';
      if (!args.containsKey(name)) continue;
      if (!used.add(name)) return {'fail': 'arg $name: fits fields $field and another'};
      named[p.simpleName] = fromWire(p.type, args[name]);
    }

    for (final name in args.keys) {
      if (!used.contains(name)) return {'fail': 'arg $name: no parameter of ${MirrorSystem.getName(mm.simpleName)} takes it'};
    }

    try {
      final result = await (m.invoke(mm.simpleName, [], named).reflectee as Future);
      final wire = toWire(result);
      save(wire, c['save'] as Map<String, dynamic>?, vars);
      return {'result': wire};
    } catch (e) {
      final em = reflect(e);
      if (em.type.instanceMembers.containsKey(#statusCode) && em.type.instanceMembers.containsKey(#body)) {
        final body = (em.getField(#body).reflectee as String?) ?? '';
        Object? model;
        try {
          model = jsonDecode(body);
        } catch (_) {}
        return {
          'error': {'status': em.getField(#statusCode).reflectee, 'body': body, if (model != null) 'model': model},
        };
      }
      return {'fail': '$e'};
    }
  }

  return {'fail': 'no method for operation ${c['op']}'};
}

Future<void> run(String inputPath, Map<String, Client> clients) async {
  final input = jsonDecode(File(inputPath).readAsStringSync()) as Map<String, dynamic>;

  for (final suite in (input['suites'] as List).cast<Map<String, dynamic>>()) {
    final client = clients[suite['name']]!;
    final vars = <String, String>{};
    final auth = Auth.fromJson(suite['auth'] as Map<String, dynamic>?);
    final cases = (suite['cases'] as List).cast<Map<String, dynamic>>();

    for (var i = 0; i < cases.length; i++) {
      final c = cases[i];
      auth.merge(Auth.fromJson(c['auth'] as Map<String, dynamic>?));

      String ex(String? s) {
        try {
          return expand(s ?? '', vars, false);
        } catch (_) {
          return s ?? '';
        }
      }

      final current = Auth.fromJson({'username': ex(auth.username), 'password': ex(auth.password), 'apiKey': ex(auth.apiKey), 'bearer': ex(auth.bearer)});

      if (suite['mock'] == true) {
        final http = HttpClient();
        final req = await http.postUrl(Uri.parse('${suite['base']}/__e2e/case/$i'));
        await (await req.close()).drain<void>();
        http.close();
      }

      Map<String, Object?> out;
      try {
        out = await callCase(client, suite, c, current, vars);
      } catch (e, st) {
        out = {'fail': 'panic: $e\n$st'};
      }

      stdout.writeln(jsonEncode({'suite': suite['name'], 'case': i, ...out}));
    }
  }
}
