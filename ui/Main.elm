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


type alias SystemAddonState =
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


joinBuiltinsUpdates : List SystemAddon -> Maybe (List SystemAddon) -> List SystemAddonState
joinBuiltinsUpdates builtins updates =
    -- TODO:
    -- [{id: "a", version: "1.0"}]
    -- [{id: "a", version: "1.5"}, {id: "b", version: "3.0"}]
    -- --> [{id: "a", builtin: "1.0", update: "1.5"},
    --      {id: "b", builtin: Nothing, update: "3.0"}]
    -- > Or better data model ?
    List.map (\a -> SystemAddonState a.id (Just a.version) Nothing) builtins


viewSystemAddonStateRow : SystemAddonState -> Html.Html Msg
viewSystemAddonStateRow addon =
    Html.tr []
        [ Html.td [] [ Html.text addon.id ]
        , Html.td [] [ Html.text <| Maybe.withDefault "" addon.builtin ]
        , Html.td [] [ Html.text <| Maybe.withDefault "" addon.update ]
        ]


viewSystemAddons : List SystemAddonState -> Html.Html Msg
viewSystemAddons addons =
    Html.table []
        [ Html.thead []
            [ Html.td [] [ Html.text "Id" ]
            , Html.td [] [ Html.text "Built-in" ]
            , Html.td [] [ Html.text "Updated" ]
            ]
        , Html.tbody [] <|
            List.map viewSystemAddonStateRow addons
        ]


viewRelease : Release -> Html.Html Msg
viewRelease release =
    Html.div []
        [ Html.h2 [] [ Html.text release.details.filename ]
        , viewReleaseDetails release.details
        , Html.dl []
            [ Html.dt [] [ Html.text "System Addons" ]
            , Html.dd [] [ viewSystemAddons <| joinBuiltinsUpdates release.builtins release.updates ]
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

        _ ->
            ( model, Cmd.none )


main =
    Html.program
        { init = init
        , subscriptions = subscriptions
        , view = view
        , update = update
        }
