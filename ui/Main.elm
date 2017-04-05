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


type alias SystemAddonVersions =
    { id : String
    , builtin : Maybe String
    , update : Maybe String
    }


type alias ReleaseDetails =
    { buildId : String
    , channel : String
    , filename : String
    , lang : String
    , target : String
    , url : String
    , version : String
    }


type alias Release =
    { builtins : List SystemAddon
    , updates : Maybe (List SystemAddon)
    , details : ReleaseDetails
    , id : String
    , last_modified : Int
    }


type alias Model =
    { releases : List Release }


type Msg
    = NoOp
    | ReleasesFetched (Result Kinto.Error (List Release))


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
        (Decode.field "updates" <| Decode.maybe <| Decode.list decodeSystemAddon)
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


init : ( Model, Cmd Msg )
init =
    ( { releases = [] }, getReleaseList )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none


viewReleaseDetails : ReleaseDetails -> Html.Html Msg
viewReleaseDetails details =
    Html.div []
        [ Html.dl []
            [ Html.dt [] [ Html.text "URL" ]
            , Html.dd [] [ Html.text details.url ]
            ]
        , Html.dl []
            [ Html.dt [] [ Html.text "Build ID" ]
            , Html.dd [] [ Html.text details.buildId ]
            ]
        , Html.dl []
            [ Html.dt [] [ Html.text "Target" ]
            , Html.dd [] [ Html.text details.target ]
            ]
        , Html.dl []
            [ Html.dt [] [ Html.text "Lang" ]
            , Html.dd [] [ Html.text details.lang ]
            ]
        , Html.dl []
            [ Html.dt [] [ Html.text "Channel" ]
            , Html.dd [] [ Html.text details.channel ]
            ]
        ]


joinBuiltinsUpdates : List SystemAddon -> Maybe (List SystemAddon) -> List SystemAddonVersions
joinBuiltinsUpdates builtins updates =
    builtinVersions builtins
        |> updateVersions updates
            |> List.sortBy .id


builtinVersions : List SystemAddon -> List SystemAddonVersions
builtinVersions builtins =
    List.map (\a -> SystemAddonVersions a.id (Just a.version) Nothing) builtins


updateVersions : Maybe (List SystemAddon) -> List SystemAddonVersions -> List SystemAddonVersions
updateVersions updates builtins =
    let
        uupdates =
            Maybe.withDefault [] updates

        mergeVersion : SystemAddonVersions -> SystemAddon -> SystemAddonVersions
        mergeVersion b u =
            SystemAddonVersions b.id b.builtin (Just u.version)

        hasBuiltin : List SystemAddonVersions -> SystemAddon -> Bool
        hasBuiltin addons addon =
            List.any (\i -> i.id == addon.id) addons

        addToBuiltin : List SystemAddonVersions -> SystemAddon -> List SystemAddonVersions
        addToBuiltin addons addon =
            List.map
                (\i ->
                    if i.id == addon.id then
                        mergeVersion i addon
                    else
                        i
                )
                addons
    in
        List.foldr
            (\a acc ->
                if hasBuiltin acc a then
                    addToBuiltin acc a
                else
                    List.append acc [ SystemAddonVersions a.id Nothing (Just a.version) ]
            )
            builtins
            uupdates


viewSystemAddonVersionsRow : SystemAddonVersions -> Html.Html Msg
viewSystemAddonVersionsRow addon =
    Html.tr []
        [ Html.td [] [ Html.text addon.id ]
        , Html.td [] [ Html.text <| Maybe.withDefault "" addon.builtin ]
        , Html.td [] [ Html.text <| Maybe.withDefault "" addon.update ]
        ]


viewSystemAddons : List SystemAddon -> Maybe (List SystemAddon) -> Html.Html Msg
viewSystemAddons builtins updates =
    Html.table []
        [ Html.thead []
            [ Html.td [] [ Html.text "Id" ]
            , Html.td [] [ Html.text "Built-in" ]
            , Html.td [] [ Html.text "Updated" ]
            ]
        , Html.tbody [] <|
            List.map viewSystemAddonVersionsRow <|
                joinBuiltinsUpdates builtins updates
        ]


viewRelease : Release -> Html.Html Msg
viewRelease { details, builtins, updates } =
    Html.div []
        [ Html.h2 [] [ Html.text details.filename ]
        , viewReleaseDetails details
        , Html.dl []
            [ Html.dt [] [ Html.text "System Addons" ]
            , Html.dd [] [ viewSystemAddons builtins updates ]
            ]
        ]


view : Model -> Html.Html Msg
view model =
    Html.div [] <|
        List.map viewRelease model.releases


update : Msg -> Model -> ( Model, Cmd Msg )
update message model =
    case message of
        ReleasesFetched result ->
            case result of
                Ok releases ->
                    ( { model | releases = releases }, Cmd.none )

                Err err ->
                    Debug.crash "crash"

        NoOp ->
            ( model, Cmd.none )


main =
    Html.program
        { init = init
        , subscriptions = subscriptions
        , view = view
        , update = update
        }