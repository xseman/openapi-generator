"""Warehouse: a strict FastAPI backend that puts every common OpenAPI shape
into one spec, for testing generated clients end to end (internal/e2e).

Every request is validated by pydantic (extra keys rejected, 422 on any
mismatch). Most endpoints echo what they parsed, so a client can check the
exact values that arrived, not only the status code."""

import datetime as dt
import uuid
from decimal import Decimal
from enum import Enum, IntEnum
from ipaddress import IPv4Address
from typing import Annotated, Any, Generic, Literal, Optional, TypeVar, Union

from fastapi import (
    APIRouter,
    Body,
    Cookie,
    Depends,
    FastAPI,
    File,
    Form,
    Header,
    HTTPException,
    Path,
    Query,
    Request,
    Response,
    Security,
    UploadFile,
)
from fastapi.responses import JSONResponse, PlainTextResponse, RedirectResponse, StreamingResponse
from fastapi.security import (
    APIKeyCookie,
    APIKeyHeader,
    APIKeyQuery,
    HTTPAuthorizationCredentials,
    HTTPBasic,
    HTTPBasicCredentials,
    HTTPBearer,
    OAuth2PasswordBearer,
    OAuth2PasswordRequestForm,
    SecurityScopes,
)
from pydantic import (
    AliasGenerator,
    AwareDatetime,
    BaseModel,
    ConfigDict,
    Discriminator,
    EmailStr,
    Field,
    HttpUrl,
    NaiveDatetime,
    SecretStr,
    Tag,
    computed_field,
)
from pydantic.alias_generators import to_camel

app = FastAPI(
    title="Warehouse",
    version="2.0.0",
    description="Every common OpenAPI shape in one strict API.",
)


# ---------------------------------------------------------------- base models


class Strict(BaseModel):
    model_config = ConfigDict(extra="forbid")


class Camel(BaseModel):
    """camelCase on the wire, snake_case in Python, as most JSON APIs."""

    model_config = ConfigDict(extra="forbid", alias_generator=AliasGenerator(to_camel), populate_by_name=True)


# ---------------------------------------------------------------- enums


class Color(str, Enum):
    red = "red"
    dark_blue = "dark-blue"
    light_green = "light green"
    n2 = "2nd"
    type_ = "type"
    empty = ""


class Priority(IntEnum):
    low = 1
    mid = 5
    high = 10


class Unit(str, Enum):
    kg = "kg"
    lb = "lb"


# ---------------------------------------------------------------- scalars


class Scalars(Strict):
    """Every scalar format pydantic writes into a schema."""

    i: int
    i64: Annotated[int, Field(ge=-(2**63), le=2**63 - 1)]
    f: float
    d: Decimal
    b: bool
    s: Annotated[str, Field(min_length=1, max_length=40, pattern=r"^[^\x00]+$")]
    when: dt.datetime
    day: dt.date
    clock: dt.time
    span: dt.timedelta
    id: uuid.UUID
    email: EmailStr
    url: HttpUrl
    ip: IPv4Address
    blob: bytes
    anything: Any = None
    color: Color
    priority: Priority
    unit: Literal["kg", "lb"] = "kg"


class Stamps(Strict):
    """Times as ORM-backed services send them: naive, aware, in containers."""

    naive: NaiveDatetime
    aware: AwareDatetime
    clock: dt.time
    span: dt.timedelta
    days: list[dt.date] = []
    seen: dict[str, dt.datetime] = {}


class Nulls(Strict):
    """Null as a required value, and null that differs from a default."""

    note: Optional[str]
    due: Optional[dt.date]
    color: Optional[Color]
    dims: Optional["Dimensions"]
    lines: Optional[list["Dimensions"]]
    payment: Optional["Payment"]
    retries: Optional[int] = 3
    extras: Optional[list["Dimensions"]] = None


class NullsEcho(Nulls):
    sent: list[str]  # the keys the client put on the wire


class FormIn(Strict):
    """A form model: every value a client must turn into text itself."""

    day: dt.date
    when: AwareDatetime
    price: Decimal
    color: Color
    tags: list[str] = []
    colors: list[Color] = []
    days: list[dt.date] = []
    qty: Optional[int] = None
    agree: bool = False


class Retry(Enum):
    """Members of two JSON types: the schema has an enum and no type."""

    never = 0
    always = "always"
    once = 1


class Literals(Strict):
    """Enums whose values are not one JSON type, null among them."""

    retry: Retry
    pin: Literal["x", None]
    order: Optional[Literal["asc", "desc"]]
    mode: Literal["auto", 0, 1] = "auto"
    tag: Literal["a", None] = "a"
    size: Literal[10, 25, 50] = 25


# ---------------------------------------------------------------- containers


class Dimensions(Camel):
    width_cm: float
    height_cm: float
    depth_cm: Optional[float] = None


class Containers(Strict):
    tags: list[str]
    unique: set[str] = set()
    pair: tuple[int, str]
    matrix: list[list[int]] = []
    sparse: list[Optional[int]] = []
    counts: dict[str, int] = {}
    groups: dict[str, list[str]] = {}
    by_name: dict[str, Dimensions] = {}
    maybe_list: Optional[list[str]] = None
    maybe_map: Optional[dict[str, Optional[str]]] = None


# ---------------------------------------------------------------- items


class Supplier(Camel):
    name: str
    contact_email: Optional[EmailStr] = None
    website: Optional[HttpUrl] = None


class ItemIn(Camel):
    """An item as a client sends it."""

    sku: Annotated[str, Field(pattern=r"^[A-Z]{3}-\d{4}$", examples=["ABC-1234"])]
    name: str
    description: Optional[str] = Field(default=None, description="Free text; null clears it.")
    price: Decimal
    weight: Optional[float] = None
    unit: Unit = Unit.kg
    color: Optional[Color] = None
    priority: Priority = Priority.mid
    tags: list[str] = []
    dimensions: Optional[Dimensions] = None
    supplier: Optional[Supplier] = None
    attributes: dict[str, str] = {}
    discontinued: bool = Field(default=False, deprecated=True)


class Item(ItemIn):
    """An item as the server returns it."""

    id: uuid.UUID
    created_at: dt.datetime
    updated_at: Optional[dt.datetime] = None
    version: int

    @computed_field
    @property
    def display_name(self) -> str:
        return f"{self.name} ({self.sku})"


class ItemPatch(Camel):
    name: Optional[str] = None
    description: Optional[str] = None
    price: Optional[Decimal] = None
    color: Optional[Color] = None
    tags: Optional[list[str]] = None
    dimensions: Optional[Dimensions] = None


T = TypeVar("T")


class Page(BaseModel, Generic[T]):
    items: list[T]
    total: int
    next_cursor: Optional[str] = None


# ---------------------------------------------------------------- unions


class Card(Strict):
    method: Literal["card"]
    last4: Annotated[str, Field(pattern=r"^\d{4}$")]
    expires: dt.date


class Transfer(Strict):
    method: Literal["transfer"]
    iban: str
    reference: Optional[str] = None


class Voucher(Strict):
    method: Literal["voucher"]
    code: str
    amount: Decimal


Payment = Annotated[Union[Card, Transfer, Voucher], Field(discriminator="method")]


class Circle(Strict):
    radius: float


class Rect(Strict):
    width: float
    height: float


class OrderLine(Camel):
    sku: str
    quantity: Annotated[int, Field(gt=0)]
    note: Optional[str] = None


class OrderIn(Camel):
    lines: Annotated[list[OrderLine], Field(min_length=1)]
    payment: Payment
    shape: Optional[Union[Circle, Rect]] = None
    gift: Union[bool, str] = False
    code: Optional[Union[int, str]] = None
    extra_payments: list[Payment] = []


class OrderStatus(str, Enum):
    placed = "placed"
    paid = "paid"
    shipped = "shipped"


class Kind(str, Enum):
    cat = "cat"
    dog = "dog"


class Cat(Camel):
    pet_kind: Literal[Kind.cat]
    lives: int


class Dog(Camel):
    pet_kind: Literal[Kind.dog]
    good_boy: bool


# Tagged by a camelCase key whose values are enum members.
Pet = Annotated[Union[Cat, Dog], Field(discriminator="pet_kind")]


def _method(v: Any) -> Optional[str]:
    return v.get("method") if isinstance(v, dict) else getattr(v, "method", None)


# A callable discriminator: the spec has a plain oneOf, no discriminator.
Tagged = Annotated[Union[Annotated[Card, Tag("card")], Annotated[Transfer, Tag("transfer")]], Discriminator(_method)]


class Wallet(Strict):
    """The union in every place a model can hold it."""

    primary: Optional[Payment] = None
    by_label: dict[str, Payment] = {}
    pets: list[Pet] = []
    tagged: Optional[Tagged] = None


class Order(OrderIn):
    id: int
    status: OrderStatus
    placed_at: dt.datetime


# ---------------------------------------------------------------- recursion, open objects, keywords


class Category(Strict):
    name: str
    children: list["Category"] = []
    parent: Optional["Category"] = None


class Open(BaseModel):
    """Takes any extra key and returns it."""

    model_config = ConfigDict(extra="allow")
    kind: str


class Keywords(BaseModel):
    """Property names that are keywords or predeclared in Go."""

    model_config = ConfigDict(extra="forbid", populate_by_name=True)
    type: str
    func: int
    range: list[int]
    map: dict[str, int]
    interface: Optional[str] = None
    package: str = "main"
    default: bool = False
    select: Optional[str] = None
    string: str = Field(alias="string")
    error_: str = Field(alias="error")
    dash_name: str = Field(alias="x-dash-name")
    dot_name: str = Field(alias="dot.name")
    upper: str = Field(alias="UPPER")
    digit: str = Field(alias="1st")


class Secret(Strict):
    username: str
    password: SecretStr


# ---------------------------------------------------------------- errors


class Problem(Strict):
    type: str
    title: str
    status: int
    detail: Optional[str] = None


class Conflict(Strict):
    message: str
    current_version: int


# ---------------------------------------------------------------- state

# Nulls names Dimensions and Payment before they are defined.
Nulls.model_rebuild()
NullsEcho.model_rebuild()

ITEMS: dict[uuid.UUID, Item] = {}
ORDERS: dict[int, Order] = {}
FILES: dict[str, bytes] = {}

# ---------------------------------------------------------------- security

oauth2 = OAuth2PasswordBearer(
    tokenUrl="token",
    scopes={"items:read": "Read items", "items:write": "Write items", "orders": "Place orders"},
)
bearer = HTTPBearer(auto_error=False)
basic = HTTPBasic()
key_header = APIKeyHeader(name="X-API-Key", auto_error=False)
key_query = APIKeyQuery(name="api_key", auto_error=False)
key_cookie = APIKeyCookie(name="session", auto_error=False)

TOKENS = {"tok-rw": ["items:read", "items:write", "orders"], "tok-r": ["items:read"]}


def user(scopes: SecurityScopes, token: str = Depends(oauth2)) -> str:
    granted = TOKENS.get(token)
    if granted is None:
        raise HTTPException(401, "bad token", headers={"WWW-Authenticate": "Bearer"})
    for s in scopes.scopes:
        if s not in granted:
            raise HTTPException(403, f"missing scope {s}")
    return token


def any_key(
    h: Optional[str] = Depends(key_header),
    q: Optional[str] = Depends(key_query),
    c: Optional[str] = Depends(key_cookie),
) -> str:
    for where, v in (("header", h), ("query", q), ("cookie", c)):
        if v == "key-1":
            return where
    raise HTTPException(401, "no api key")


def admin(c: HTTPBasicCredentials = Depends(basic)) -> str:
    if (c.username, c.password) != ("admin", "p@ss:w0rd"):
        raise HTTPException(401, "bad credentials")
    return c.username


def maybe_bearer(c: Optional[HTTPAuthorizationCredentials] = Depends(bearer)) -> Optional[str]:
    return c.credentials if c else None


def problem(status: int, title: str, detail: str = "") -> JSONResponse:
    return JSONResponse({"type": "about:blank", "title": title, "status": status, "detail": detail or None}, status)


not_found = {404: {"model": Problem, "description": "No such resource"}}

# ---------------------------------------------------------------- auth


@app.post("/token", tags=["auth"], operation_id="issueToken")
def issue_token(form: Annotated[OAuth2PasswordRequestForm, Depends()]) -> dict[str, str]:
    if (form.username, form.password) == ("alice", "secret"):
        return {"access_token": "tok-rw", "token_type": "bearer", "scope": " ".join(form.scopes)}
    if (form.username, form.password) == ("bob", "secret"):
        return {"access_token": "tok-r", "token_type": "bearer", "scope": " ".join(form.scopes)}
    raise HTTPException(400, "bad login")


@app.get("/whoami", tags=["auth"], operation_id="whoami")
def whoami(where: str = Depends(any_key), token: Optional[str] = Depends(maybe_bearer)) -> dict[str, Optional[str]]:
    return {"key_in": where, "bearer": token}


@app.get("/admin/stats", tags=["auth"], operation_id="adminStats")
def admin_stats(_: str = Depends(admin)) -> dict[str, int]:
    return {"items": len(ITEMS), "orders": len(ORDERS)}


# ---------------------------------------------------------------- echo: scalars and containers


@app.post("/echo/scalars", tags=["echo"], operation_id="echoScalars")
def echo_scalars(body: Scalars) -> Scalars:
    return body


@app.post("/echo/literals", tags=["echo"], operation_id="echoLiterals")
def echo_literals(body: Literals) -> Literals:
    return body


@app.post("/echo/nulls", tags=["echo"], operation_id="echoNulls")
def echo_nulls(body: Nulls) -> NullsEcho:
    return NullsEcho(**body.model_dump(), sent=sorted(body.model_fields_set))


@app.post("/echo/stamps", tags=["echo"], operation_id="echoStamps")
def echo_stamps(body: Stamps) -> Stamps:
    return body


@app.get("/stamps", tags=["echo"], operation_id="stampsQuery")
def stamps_query(
    days: Annotated[list[dt.date], Query()] = [],
    since: Optional[NaiveDatetime] = None,
) -> Stamps:
    return Stamps(
        naive=since or dt.datetime(2024, 3, 1, 10, 0, 0, 500000),
        aware=dt.datetime(2024, 3, 1, 10, tzinfo=dt.timezone.utc),
        clock=dt.time(13, 45, 30, 250000),
        span=dt.timedelta(seconds=-1.5),
        days=days,
    )


@app.post("/echo/containers", tags=["echo"], operation_id="echoContainers")
def echo_containers(body: Containers) -> Containers:
    return body


@app.post("/echo/keywords", tags=["echo"], operation_id="echoKeywords")
def echo_keywords(body: Keywords) -> Keywords:
    return body


@app.post("/echo/open", tags=["echo"], operation_id="echoOpen")
def echo_open(body: Open) -> Open:
    return body


@app.post("/echo/tree", tags=["echo"], operation_id="echoTree")
def echo_tree(body: Category) -> Category:
    return body


@app.post("/echo/secret", tags=["echo"], operation_id="echoSecret")
def echo_secret(body: Secret) -> dict[str, Union[str, int]]:
    return {"username": body.username, "password_length": len(body.password.get_secret_value())}


@app.post("/echo/list", tags=["echo"], operation_id="echoList")
def echo_list(body: list[Dimensions]) -> list[Dimensions]:
    return body


@app.post("/echo/map", tags=["echo"], operation_id="echoMap")
def echo_map(body: dict[str, list[int]]) -> dict[str, int]:
    return {k: sum(v) for k, v in body.items()}


@app.post("/echo/number", tags=["echo"], operation_id="echoNumber")
def echo_number(body: Annotated[float, Body()]) -> float:
    return body * 2


@app.post("/echo/text", tags=["echo"], operation_id="echoText", response_class=PlainTextResponse,
          openapi_extra={"requestBody": {"required": True, "content": {"text/plain": {"schema": {"type": "string"}}}}})
async def echo_text(request: Request) -> str:
    return (await request.body()).decode()[::-1]


@app.post("/echo/raw", tags=["echo"], operation_id="echoRaw",
          openapi_extra={"requestBody": {"required": True, "content": {"application/octet-stream": {"schema": {"type": "string", "format": "binary"}}}}},
          responses={200: {"content": {"application/octet-stream": {"schema": {"type": "string", "format": "binary"}}}}},
          response_class=Response)
async def echo_raw(request: Request) -> Response:
    data = await request.body()
    return Response(bytes(reversed(data)), media_type="application/octet-stream")


@app.post("/echo/union", tags=["echo"], operation_id="echoUnion")
def echo_union(body: Union[int, str, list[int]]) -> dict[str, str]:
    return {"type": type(body).__name__, "value": str(body)}


@app.post("/echo/pet", tags=["echo"], operation_id="echoPet")
def echo_pet(body: Annotated[Pet, Body()]) -> Pet:
    return body


@app.post("/echo/wallet", tags=["echo"], operation_id="echoWallet")
def echo_wallet(body: Wallet) -> Wallet:
    return body


@app.post("/echo/optional", tags=["echo"], operation_id="echoOptional")
def echo_optional(body: Annotated[Optional[Dimensions], Body()] = None) -> dict[str, bool]:
    return {"got": body is not None}


# ---------------------------------------------------------------- parameters


@app.get("/params/path/{id}/{day}/{color}/{rest:path}", tags=["params"], operation_id="pathParams")
def path_params(id: int, day: dt.date, color: Color, rest: str) -> dict[str, str]:
    return {"id": str(id), "day": day.isoformat(), "color": color.value, "rest": rest}


@app.get("/params/query", tags=["params"], operation_id="queryParams")
def query_params(
    q: str,
    page_size: Annotated[int, Query(alias="page-size", ge=1, le=100)] = 20,
    tags: Annotated[list[str], Query()] = [],
    ids: Annotated[Optional[list[int]], Query()] = None,
    colors: Annotated[list[Color], Query()] = [],
    since: Optional[dt.datetime] = None,
    ratio: Optional[float] = None,
    active: Optional[bool] = None,
    old: Annotated[Optional[str], Query(deprecated=True)] = None,
) -> dict[str, Any]:
    return {
        "q": q,
        "pageSize": page_size,
        "tags": tags,
        "ids": ids,
        "colors": [c.value for c in colors],
        "since": since.isoformat() if since else None,
        "ratio": ratio,
        "active": active,
        "old": old,
    }


@app.get("/params/headers", tags=["params"], operation_id="headerParams")
def header_params(
    x_request_id: Annotated[uuid.UUID, Header()],
    x_retry: Annotated[Optional[int], Header()] = None,
    x_flags: Annotated[list[str], Header()] = [],
    accept_language: Annotated[Optional[str], Header()] = None,
    tenant: Annotated[Optional[str], Cookie()] = None,
    theme: Annotated[Optional[str], Cookie()] = None,
) -> dict[str, Any]:
    return {
        "requestId": str(x_request_id),
        "retry": x_retry,
        "flags": x_flags,
        "lang": accept_language,
        "tenant": tenant,
        "theme": theme,
    }


# ---------------------------------------------------------------- items CRUD


@app.post(
    "/items",
    status_code=201,
    tags=["items"],
    operation_id="createItem",
    responses={
        201: {"headers": {"Location": {"schema": {"type": "string"}, "description": "URL of the item"}}},
        409: {"model": Problem, "description": "SKU taken"},
    },
)
def create_item(body: ItemIn, response: Response, _: str = Security(user, scopes=["items:write"])) -> Item:
    if any(i.sku == body.sku for i in ITEMS.values()):
        return problem(409, "SKU taken", body.sku)
    item = Item(id=uuid.uuid4(), created_at=dt.datetime.now(dt.timezone.utc), version=1, **body.model_dump())
    ITEMS[item.id] = item
    response.headers["Location"] = f"/items/{item.id}"
    return item


@app.get("/items", tags=["items"], operation_id="listItems")
def list_items(
    color: Optional[Color] = None,
    min_price: Optional[Decimal] = None,
    cursor: Optional[str] = None,
    limit: Annotated[int, Query(ge=1, le=50)] = 10,
    _: str = Security(user, scopes=["items:read"]),
) -> Page[Item]:
    items = sorted(ITEMS.values(), key=lambda i: i.sku)
    items = [i for i in items if (color is None or i.color == color) and (min_price is None or i.price >= min_price)]
    start = int(cursor or 0)
    chunk = items[start : start + limit]
    nxt = str(start + limit) if start + limit < len(items) else None
    return Page[Item](items=chunk, total=len(items), next_cursor=nxt)


@app.get("/items/{item_id}", tags=["items"], operation_id="getItem", responses=not_found)
def get_item(item_id: uuid.UUID, _: str = Security(user, scopes=["items:read"])) -> Item:
    if item_id not in ITEMS:
        return problem(404, "Not found", str(item_id))
    return ITEMS[item_id]


@app.patch(
    "/items/{item_id}",
    tags=["items"],
    operation_id="patchItem",
    responses={**not_found, 409: {"model": Conflict, "description": "Stale version"}},
)
def patch_item(
    item_id: uuid.UUID,
    body: ItemPatch,
    if_match: Annotated[int, Header()],
    _: str = Security(user, scopes=["items:write"]),
) -> Item:
    if item_id not in ITEMS:
        return problem(404, "Not found", str(item_id))
    cur = ITEMS[item_id]
    if if_match != cur.version:
        return JSONResponse({"message": "stale", "current_version": cur.version}, 409)
    data = cur.model_dump()
    sent = body.model_dump(exclude_unset=True)
    for k in ("name", "price"):
        if k in sent and sent[k] is None:
            raise HTTPException(400, f"{k} may not be null")
    data.update(sent)
    data.update(version=cur.version + 1, updated_at=dt.datetime.now(dt.timezone.utc))
    data.pop("display_name", None)
    ITEMS[item_id] = Item(**data)
    return ITEMS[item_id]


@app.delete("/items/{item_id}", status_code=204, tags=["items"], operation_id="deleteItem", responses=not_found)
def delete_item(item_id: uuid.UUID, _: str = Security(user, scopes=["items:write"])):
    if ITEMS.pop(item_id, None) is None:
        return problem(404, "Not found", str(item_id))
    return Response(status_code=204)


@app.head("/items/{item_id}", tags=["items"], operation_id="itemExists", responses={404: {"description": "absent"}})
def item_exists(item_id: uuid.UUID, _: str = Security(user, scopes=["items:read"])):
    return Response(status_code=200 if item_id in ITEMS else 404)


# ---------------------------------------------------------------- files


@app.post("/files", status_code=201, tags=["files"], operation_id="uploadFiles")
def upload_files(
    files: Annotated[list[UploadFile], File(description="One or more files")],
    folder: Annotated[str, Form()],
    labels: Annotated[list[str], Form()] = [],
    overwrite: Annotated[bool, Form()] = False,
    meta: Annotated[Optional[str], Form(description="JSON text")] = None,
    _: str = Depends(any_key),
) -> list[dict[str, Union[str, int]]]:
    out = []
    for f in files:
        data = f.file.read()
        name = f"{folder}/{f.filename}"
        if name in FILES and not overwrite:
            raise HTTPException(409, f"{name} exists")
        FILES[name] = data
        out.append({"name": name, "size": len(data), "type": f.content_type or "", "labels": "|".join(labels), "meta": meta or ""})
    return out


@app.post("/forms/typed", tags=["files"], operation_id="submitForm")
def submit_form(body: Annotated[FormIn, Form()]) -> FormIn:
    return body


@app.post("/files/avatar", tags=["files"], operation_id="uploadAvatar")
def upload_avatar(image: Annotated[UploadFile, File()], thumbnail: Annotated[Optional[UploadFile], File()] = None) -> dict[str, int]:
    return {"image": len(image.file.read()), "thumbnail": len(thumbnail.file.read()) if thumbnail else -1}


@app.get(
    "/files/{name:path}",
    tags=["files"],
    operation_id="downloadFile",
    response_class=Response,
    responses={200: {"content": {"application/octet-stream": {"schema": {"type": "string", "format": "binary"}}}}, **not_found},
)
def download_file(name: str):
    if name not in FILES:
        return problem(404, "Not found", name)
    return Response(FILES[name], media_type="application/octet-stream")


@app.get("/export.csv", tags=["files"], operation_id="exportCsv", response_class=PlainTextResponse,
         responses={200: {"content": {"text/csv": {"schema": {"type": "string"}}}}})
def export_csv() -> PlainTextResponse:
    rows = ["sku,name"] + [f"{i.sku},{i.name}" for i in sorted(ITEMS.values(), key=lambda i: i.sku)]
    return PlainTextResponse("\n".join(rows) + "\n", media_type="text/csv")


# ---------------------------------------------------------------- orders


@app.post(
    "/orders",
    status_code=201,
    tags=["orders"],
    operation_id="placeOrder",
    responses={202: {"model": Problem, "description": "Queued for review"}, 422: {"description": "Invalid"}},
)
def place_order(
    body: OrderIn,
    dry_run: bool = False,
    idempotency_key: Annotated[Optional[str], Header()] = None,
    _: str = Security(user, scopes=["orders"]),
) -> Order:
    total = sum(line.quantity for line in body.lines)
    if total > 100:
        return problem(202, "Queued for review", f"{total} units")
    order = Order(id=len(ORDERS) + 1, status=OrderStatus.placed, placed_at=dt.datetime.now(dt.timezone.utc), **body.model_dump())
    if not dry_run:
        ORDERS[order.id] = order
    return order


@app.get("/orders/{order_id}", tags=["orders"], operation_id="getOrder", responses=not_found)
def get_order(order_id: Annotated[int, Path(ge=1)], _: str = Security(user, scopes=["orders"])) -> Order:
    if order_id not in ORDERS:
        return problem(404, "Not found", str(order_id))
    return ORDERS[order_id]


@app.get("/orders/{order_id}/status", tags=["orders"], operation_id="orderStatus", responses=not_found)
def order_status(order_id: int, _: str = Security(user, scopes=["orders"])) -> OrderStatus:
    if order_id not in ORDERS:
        return problem(404, "Not found", str(order_id))
    return ORDERS[order_id].status


@app.post("/orders/{order_id}/cancel", status_code=202, tags=["orders"], operation_id="cancelOrder",
          responses={202: {"description": "Cancellation accepted; no body"}, **not_found})
def cancel_order(order_id: int, reason: Annotated[Optional[str], Body(embed=True)] = None, _: str = Security(user, scopes=["orders"])):
    if order_id not in ORDERS:
        return problem(404, "Not found", str(order_id))
    return Response(status_code=202)


# ---------------------------------------------------------------- misc


@app.get("/categories", tags=["misc"], operation_id="categoryTree")
def category_tree() -> list[Category]:
    root = Category(name="root", children=[Category(name="a", children=[Category(name="a1")]), Category(name="b")])
    return [root]


@app.get("/count", tags=["misc"], operation_id="countItems")
def count_items() -> int:
    return len(ITEMS)


@app.get("/maybe", tags=["misc"], operation_id="maybeItem")
def maybe_item(present: bool = False) -> Optional[Dimensions]:
    return Dimensions(width_cm=1, height_cm=2) if present else None


@app.get("/legacy", tags=["misc"], deprecated=True)
def legacy_endpoint(x: int = 1) -> dict[str, int]:
    """A deprecated operation with FastAPI's generated operationId."""
    return {"x": x}


@app.get("/redirect", tags=["misc"], operation_id="redirectToCount", response_class=RedirectResponse, status_code=307)
def redirect() -> RedirectResponse:
    return RedirectResponse("/count", status_code=307)


@app.get("/stream", tags=["misc"], operation_id="streamLines",
         responses={200: {"content": {"application/x-ndjson": {"schema": {"type": "string"}}}}})
def stream_lines(n: int = 3) -> StreamingResponse:
    return StreamingResponse((f'{{"i":{i}}}\n' for i in range(n)), media_type="application/x-ndjson")


@app.get("/numbers/{a}/{b}", tags=["misc"], operation_id="add")
def add(a: int, b: float, round_: Annotated[bool, Query(alias="round")] = False) -> float:
    return round(a + b) if round_ else a + b


@app.get("/untagged", operation_id="untagged")
def untagged() -> dict[str, bool]:
    return {"ok": True}


# ---------------------------------------------------------------- name clashes
# Parameters that differ on the wire but share a name once camelCased, and
# ones named like a client's own locals; a router prefix and a shared
# dependency bring them in as real services do.


def api_version(version: Annotated[str, Header(alias="Version")] = "1") -> str:
    return version


clash = APIRouter(prefix="/tenants/{tenant}", tags=["clash"])


@clash.get("/things/{item_id}", operation_id="clashLocations")
def clash_locations(
    tenant: str,
    item_id: int,
    item_hdr: Annotated[Optional[str], Header(alias="Item-Id")] = None,
    hv: str = Depends(api_version),
    version: Optional[int] = None,
) -> dict[str, Any]:
    return {"tenant": tenant, "path": item_id, "header": item_hdr, "hversion": hv, "qversion": version}


@clash.get("/sorted", operation_id="clashAliases")
def clash_aliases(
    tenant: str,
    old: Annotated[Optional[str], Query(alias="sort_by", deprecated=True)] = None,
    new: Annotated[Optional[str], Query(alias="sortBy")] = None,
) -> dict[str, Optional[str]]:
    return {"sort_by": old, "sortBy": new}


@clash.post("/notes", operation_id="clashForm")
def clash_form(
    tenant: str,
    q: Annotated[str, Query(alias="folder_name")],
    f: Annotated[str, Form(alias="folderName")],
) -> dict[str, str]:
    return {"query": q, "form": f}


@app.get("/fetch", tags=["clash"], operation_id="clashLocals")
def clash_locals(url: str, headers: Optional[str] = None, path: Optional[str] = None, query: Optional[str] = None) -> dict[str, Optional[str]]:
    return {"url": url, "headers": headers, "path": path, "query": query}


app.include_router(clash)
