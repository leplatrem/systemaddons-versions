module Model
    exposing
        ( Model
        , Release
        , SystemAddon
        , SystemAddonVersions
        , ReleaseDetails
        , FilterSet
        , Msg(..)
        , filterReleases
        , init
        , update
        )

import Dict
import Kinto
import Json.Decode as Decode


type alias Channel =
    String


type alias Lang =
    String


type alias Target =
    String


type alias Version =
    String


type Msg
    = ReleasesFetched (Result Kinto.Error (List Release))
    | ToggleChannelFilter Channel Bool
    | ToggleLangFilter Lang Bool
    | ToggleTargetFilter Target Bool
    | ToggleVersionFilter Version Bool


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


type alias FilterSet =
    Dict.Dict String Bool


type alias Filters =
    { channels : FilterSet
    , langs : FilterSet
    , targets : FilterSet
    , versions : FilterSet
    }


type alias Model =
    { releases : List Release
    , filters : Filters
    }


init : ( Model, Cmd Msg )
init =
    { releases = []
    , filters =
        { channels = Dict.fromList []
        , langs = Dict.fromList []
        , targets = Dict.fromList []
        , versions = Dict.fromList []
        }
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


extractFilterSet : (Release -> String) -> List Release -> Dict.Dict String Bool
extractFilterSet accessor releases =
    List.map (accessor >> (\c -> ( c, True ))) releases
        |> Dict.fromList


extractVersions : List Release -> Dict.Dict String Bool
extractVersions releases =
    -- TODO: sort desc
    let
        extractVersion ( version, x ) =
            case (String.split "." version) of
                v :: _ ->
                    ( v, x )

                _ ->
                    ( "None", x )
    in
        extractFilterSet (.details >> .filename) releases
            |> Dict.toList
            |> List.map extractVersion
            |> Dict.fromList


processFilter : FilterSet -> (Release -> String) -> List Release -> List Release
processFilter filterSet accessor releases =
    List.filter
        (\release ->
            Dict.get (accessor release) filterSet
                |> Maybe.withDefault True
        )
        releases


processFilterVersions : FilterSet -> List Release -> List Release
processFilterVersions filterSet releases =
    List.filter
        (\{ details } ->
            List.any
                (\f -> String.startsWith f details.filename)
                (Dict.filter (\k v -> v) filterSet |> Dict.keys)
        )
        releases


filterReleases : Model -> List Release
filterReleases { filters, releases } =
    releases
        |> processFilter filters.channels (.details >> .channel)
        |> processFilter filters.langs (.details >> .lang)
        |> processFilter filters.targets (.details >> .target)
        |> processFilterVersions filters.versions


update : Msg -> Model -> ( Model, Cmd Msg )
update message ({ filters } as model) =
    case message of
        ReleasesFetched result ->
            case result of
                Ok releases ->
                    { model
                        | releases = releases
                        , filters =
                            { channels = extractFilterSet (.details >> .channel) releases
                            , langs = extractFilterSet (.details >> .lang) releases
                            , targets = extractFilterSet (.details >> .target) releases
                            , versions = extractVersions releases
                            }
                    }
                        ! []

                Err err ->
                    Debug.crash "Unhandled Kinto error"

        ToggleChannelFilter channel active ->
            let
                channels =
                    Dict.update channel (\_ -> Just active) model.filters.channels

                newFilters =
                    { filters | channels = channels }
            in
                { model | filters = newFilters } ! []

        ToggleLangFilter lang active ->
            let
                newFilters =
                    { filters
                        | langs = Dict.update lang (\_ -> Just active) filters.langs
                    }
            in
                { model | filters = newFilters } ! []

        ToggleTargetFilter target active ->
            let
                newFilters =
                    { filters
                        | targets = Dict.update target (\_ -> Just active) filters.targets
                    }
            in
                { model | filters = newFilters } ! []

        ToggleVersionFilter version active ->
            let
                newFilters =
                    { filters
                        | versions = Dict.update version (\_ -> Just active) filters.versions
                    }
            in
                { model | filters = newFilters } ! []
