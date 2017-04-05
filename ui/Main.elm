module Main exposing (..)

import Kinto

import Json.Decode as Decode

import Html
import Html.Attributes
import Html.Events


type alias SystemAddon =
    { id : String
    , version : String
    }

type alias Release =
    { buildId : String
    , channel : String
    , filename : String
    , lang : String
    , target : String
    , url : String
    , version : String
    }

type alias ReleaseInfo =
    { builtins: List SystemAddon
    , updates: Maybe (List SystemAddon)
    , release: Release
    , id: String
    , last_modified: Int
    }

type alias Model =
    { releases: List ReleaseInfo }


type Msg
    = NoOp
    | ReleasesFetched (Result Kinto.Error (List ReleaseInfo))

decodeSystemAddon :  Decode.Decoder SystemAddon
decodeSystemAddon =
    Decode.map2 SystemAddon
        (Decode.field "id" Decode.string)
        (Decode.field "version" Decode.string)

decodeRelease : Decode.Decoder Release
decodeRelease =
    Decode.map7 Release
        (Decode.field "buildId" Decode.string)
        (Decode.field "channel" Decode.string)
        (Decode.field "filename" Decode.string)
        (Decode.field "lang" Decode.string)
        (Decode.field "target" Decode.string)
        (Decode.field "url" Decode.string)
        (Decode.field "version" Decode.string)

decodeReleaseInfo : Decode.Decoder ReleaseInfo
decodeReleaseInfo =
    Decode.map5 ReleaseInfo
        (Decode.field "builtins" <| Decode.list decodeSystemAddon)
        (Decode.field "updates" <| Decode.maybe <| Decode.list decodeSystemAddon)
        (Decode.field "release" decodeRelease)
        (Decode.field "id" Decode.string)
        (Decode.field "last_modified" Decode.int)

client : Kinto.Client
client =
    Kinto.client
        "https://kinto-ota.dev.mozaws.net/v1/"
        (Kinto.Basic "user" "pass")  -- XXX: useless

recordResource : Kinto.Resource ReleaseInfo
recordResource =
    Kinto.recordResource "systemaddons" "versions" decodeReleaseInfo

getReleaseList : Cmd Msg
getReleaseList =
    client
        |> Kinto.getList recordResource
        |> Kinto.sortBy [ "-release.version" ]
        |> Kinto.send ReleasesFetched


init : ( Model, Cmd Msg )
init =
    ( { releases = [] }, getReleaseList )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none


viewRelease : ReleaseInfo -> Html.Html Msg
viewRelease releaseInfo =
    Html.li []
        [ Html.text releaseInfo.id ]

view : Model -> Html.Html Msg
view model =
    Html.ul []
        <| List.map viewRelease model.releases


update : Msg -> Model -> ( Model, Cmd Msg )
update message model =
    case message of
        ReleasesFetched result ->
            case result of
                Ok releases ->
                    ( { model | releases = releases }, Cmd.none )
                Err err ->
                    Debug.crash "crash"
        _ ->
            ( model, Cmd.none )


main =
    Html.program
        { init = init
        , subscriptions = subscriptions
        , view = view
        , update = update
        }
