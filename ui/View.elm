module View exposing (view)

import Dict
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (..)
import Model
    exposing
        ( Model
        , Release
        , SystemAddon
        , SystemAddonVersions
        , ReleaseDetails
        , Msg(..)
        , filterReleases
        )


viewReleaseDetails : ReleaseDetails -> Html Msg
viewReleaseDetails details =
    div []
        [ dl []
            [ dt [] [ text "URL" ]
            , dd [] [ text details.url ]
            ]
        , dl []
            [ dt [] [ text "Build ID" ]
            , dd [] [ text details.buildId ]
            ]
        , dl []
            [ dt [] [ text "Target" ]
            , dd [] [ text details.target ]
            ]
        , dl []
            [ dt [] [ text "Lang" ]
            , dd [] [ text details.lang ]
            ]
        , dl []
            [ dt [] [ text "Channel" ]
            , dd [] [ text details.channel ]
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


viewSystemAddonVersionsRow : SystemAddonVersions -> Html Msg
viewSystemAddonVersionsRow addon =
    tr []
        [ td [] [ text addon.id ]
        , td [] [ text <| Maybe.withDefault "" addon.builtin ]
        , td [] [ text <| Maybe.withDefault "" addon.update ]
        ]


viewSystemAddons : List SystemAddon -> Maybe (List SystemAddon) -> Html Msg
viewSystemAddons builtins updates =
    table []
        [ thead []
            [ td [] [ text "Id" ]
            , td [] [ text "Built-in" ]
            , td [] [ text "Updated" ]
            ]
        , tbody [] <|
            List.map viewSystemAddonVersionsRow <|
                joinBuiltinsUpdates builtins updates
        ]


viewRelease : Release -> Html Msg
viewRelease { details, builtins, updates } =
    div []
        [ h2 [] [ text details.filename ]
        , viewReleaseDetails details
        , dl []
            [ dt [] [ text "System Addons" ]
            , dd [] [ viewSystemAddons builtins updates ]
            ]
        ]


viewFilters : Model -> Html Msg
viewFilters model =
    let
        channelFilter ( channel, active ) =
            li []
                [ label []
                    [ input
                        [ type_ "checkbox"
                        , value channel
                        , onCheck <| ToggleChannelFilter channel
                        , checked active
                        ]
                        []
                    , text channel
                    ]
                ]

        channels =
            model.filters.channels |> Dict.toList |> List.map channelFilter
    in
        div []
            [ h2 [] [ text "Filters" ]
            , h3 [] [ text "Channels" ]
            , ul [] <| channels
            ]


view : Model -> Html Msg
view model =
    div []
        [ viewFilters model
        , div [] <| List.map viewRelease <| filterReleases model
        ]
