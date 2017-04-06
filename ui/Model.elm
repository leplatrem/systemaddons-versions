module Model
    exposing
        ( Model
        , Release
        , SystemAddon
        , SystemAddonVersions
        , ReleaseDetails
        , Msg(..)
        , filterReleases
        , init
        , update
        )

import Dict
import Kinto
import Json.Decode as Decode


type Msg
    = ReleasesFetched (Result Kinto.Error (List Release))
    | ToggleChannelFilter Channel Bool


type alias Channel =
    String


type alias SystemAddon =
    { id : String
    , version : String
    }


type alias SystemAddonVersions =
    { id : String
    , builtin : Maybe String
    , update : Maybe String
    }


type alias ReleaseDetails =
    { buildId : String
    , channel : Channel
    , filename : String
    , lang : String
    , target : String
    , url : String
    , version : String
    }


type alias Release =
    { builtins : List SystemAddon
    , updates : List SystemAddon
    , details : ReleaseDetails
    , id : String
    , last_modified : Int
    }


type alias Filters =
    { channels : Dict.Dict Channel Bool
    }


type alias Model =
    { releases : List Release
    , filters : Filters
    }


init : ( Model, Cmd Msg )
init =
    { releases = []
    , filters = { channels = Dict.fromList [] }
    }
        ! [ getReleaseList ]


decodeSystemAddon : Decode.Decoder SystemAddon
decodeSystemAddon =
    Decode.map2 SystemAddon
        (Decode.field "id" Decode.string)
        (Decode.field "version" Decode.string)


decodeReleaseDetails : Decode.Decoder ReleaseDetails
decodeReleaseDetails =
    Decode.map7 ReleaseDetails
        (Decode.field "buildId" Decode.string)
        (Decode.field "channel" Decode.string)
        (Decode.field "filename" Decode.string)
        (Decode.field "lang" Decode.string)
        (Decode.field "target" Decode.string)
        (Decode.field "url" Decode.string)
        (Decode.field "version" Decode.string)


decodeRelease : Decode.Decoder Release
decodeRelease =
    Decode.map5 Release
        (Decode.field "builtins" <| Decode.list decodeSystemAddon)
        (Decode.field "updates" <| Decode.list decodeSystemAddon)
        (Decode.field "release" decodeReleaseDetails)
        (Decode.field "id" Decode.string)
        (Decode.field "last_modified" Decode.int)


client : Kinto.Client
client =
    Kinto.client
        "https://kinto-ota.dev.mozaws.net/v1/"
        (Kinto.Basic "user" "pass")


recordResource : Kinto.Resource Release
recordResource =
    Kinto.recordResource "systemaddons" "versions" decodeRelease


getReleaseList : Cmd Msg
getReleaseList =
    client
        |> Kinto.getList recordResource
        |> Kinto.sortBy [ "-release.version" ]
        |> Kinto.send ReleasesFetched


extractChannels : List Release -> Dict.Dict Channel Bool
extractChannels releaseList =
    List.map (.details >> .channel >> (\c -> ( c, True ))) releaseList
        |> Dict.fromList


filterReleases : Model -> List Release
filterReleases { filters, releases } =
    List.filter
        (\{ details } ->
            Dict.get details.channel filters.channels
                |> Maybe.withDefault True
        )
        releases


update : Msg -> Model -> ( Model, Cmd Msg )
update message model =
    case message of
        ReleasesFetched result ->
            case result of
                Ok releases ->
                    { model
                        | releases = releases
                        , filters = { channels = extractChannels releases }
                    }
                        ! []

                Err err ->
                    Debug.crash "Unhandled Kinto error"

        ToggleChannelFilter channel active ->
            { model
                | filters =
                    { channels =
                        Dict.update channel (\_ -> Just active) model.filters.channels
                    }
            }
                ! []
