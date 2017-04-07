module Model
    exposing
        ( Model
        , Release
        , SystemAddon
        , SystemAddonVersions
        , ReleaseDetails
        , FilterSet
        , Msg(..)
        , ToggleFilterMsg(..)
        , applyFilters
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


type ToggleFilterMsg
    = ToggleChannel Channel Bool
    | ToggleLang Lang Bool
    | ToggleTarget Target Bool
    | ToggleVersion Version Bool


type Msg
    = ReleasesFetched (Result Kinto.Error (List Release))
    | ToggleFilter ToggleFilterMsg


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


extractFilterSet : (Release -> String) -> List Release -> FilterSet
extractFilterSet accessor releases =
    List.map (accessor >> (\c -> ( c, True ))) releases
        |> Dict.fromList


extractVersionFilterSet : List Release -> FilterSet
extractVersionFilterSet releases =
    -- TODO: sort desc
    let
        extractVersion ( version, active ) =
            case (String.split "." version) of
                major :: _ ->
                    ( major, active )

                _ ->
                    ( version, active )
    in
        extractFilterSet (.details >> .version) releases
            |> Dict.toList
            |> List.map extractVersion
            |> Dict.fromList


applyFilter : FilterSet -> (Release -> String) -> List Release -> List Release
applyFilter filterSet accessor releases =
    List.filter
        (\release ->
            Dict.get (accessor release) filterSet
                |> Maybe.withDefault True
        )
        releases


applyVersionFilter : FilterSet -> List Release -> List Release
applyVersionFilter filterSet releases =
    List.filter
        (\{ details } ->
            List.any
                (\major -> String.startsWith major details.version)
                (Dict.filter (\k v -> v) filterSet |> Dict.keys)
        )
        releases


applyFilters : Model -> List Release
applyFilters { filters, releases } =
    releases
        |> applyFilter filters.channels (.details >> .channel)
        |> applyFilter filters.langs (.details >> .lang)
        |> applyFilter filters.targets (.details >> .target)
        |> applyVersionFilter filters.versions


toggleFilter : FilterSet -> String -> Bool -> FilterSet
toggleFilter filterSet name active =
    Dict.update name (\_ -> Just active) filterSet


updateFilters : ToggleFilterMsg -> Filters -> Filters
updateFilters toggleMsg filters =
    case toggleMsg of
        ToggleChannel channel active ->
            { filters | channels = toggleFilter filters.channels channel active }

        ToggleLang lang active ->
            { filters | langs = toggleFilter filters.langs lang active }

        ToggleTarget target active ->
            { filters | targets = toggleFilter filters.targets target active }

        ToggleVersion version active ->
            { filters | versions = toggleFilter filters.versions version active }


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
                            , versions = extractVersionFilterSet releases
                            }
                    }
                        ! []

                Err err ->
                    Debug.crash "Unhandled Kinto error"

        ToggleFilter msg ->
            { model | filters = updateFilters msg filters } ! []
