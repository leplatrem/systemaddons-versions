module View exposing (view)

import Date
import Dict
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (..)
import Date.Format
import Model
    exposing
        ( Model
        , FilterSet
        , Release
        , SystemAddon
        , SystemAddonVersions
        , ReleaseDetails
        , Msg(..)
        , ToggleFilterMsg(..)
        , applyFilters
        )


joinBuiltinsUpdates : List SystemAddon -> List SystemAddon -> List SystemAddonVersions
joinBuiltinsUpdates builtins updates =
    builtinVersions builtins
        |> updateVersions updates
        |> List.sortBy .id


builtinVersions : List SystemAddon -> List SystemAddonVersions
builtinVersions builtins =
    List.map (\a -> SystemAddonVersions a.id (Just a.version) Nothing) builtins


updateVersions : List SystemAddon -> List SystemAddonVersions -> List SystemAddonVersions
updateVersions updates builtins =
    let
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
            updates


viewReleaseDetails : ReleaseDetails -> Html Msg
viewReleaseDetails details =
    table [ class "table table-condensed" ]
        [ thead []
            [ tr []
                [ th [] [ text "Build ID" ]
                , th [] [ text "Target" ]
                , th [] [ text "Lang" ]
                , th [] [ text "Version" ]
                , th [] [ text "Channel" ]
                , th [] [ text "URL" ]
                ]
            ]
        , tbody []
            [ tr []
                [ td [] [ text details.buildId ]
                , td [] [ text details.target ]
                , td [] [ text details.lang ]
                , td [] [ text details.version ]
                , td [] [ text details.channel ]
                , td [] [ a [ href details.url, title details.url ] [ text details.filename ] ]
                ]
            ]
        ]


viewSystemAddonVersionsRow : SystemAddonVersions -> Html Msg
viewSystemAddonVersionsRow addon =
    tr []
        [ td [] [ text addon.id ]
        , td [] [ text <| Maybe.withDefault "" addon.builtin ]
        , td [] [ text <| Maybe.withDefault "" addon.update ]
        ]


viewSystemAddons : List SystemAddon -> List SystemAddon -> Html Msg
viewSystemAddons builtins updates =
    table [ class "table table-stripped table-condensed" ]
        [ thead []
            [ tr []
                [ th [] [ text "Id" ]
                , th [] [ text "Built-in" ]
                , th [] [ text "Updated" ]
                ]
            ]
        , tbody [] <|
            List.map viewSystemAddonVersionsRow <|
                joinBuiltinsUpdates builtins updates
        ]


formatDate : Int -> String
formatDate last_modified =
    Date.fromTime (toFloat last_modified)
        |> Date.Format.format "%Y/%m/%d %H:%k:%S"


viewRelease : Release -> Html Msg
viewRelease { id, details, builtins, updates, last_modified } =
    div [ class "panel panel-default", Html.Attributes.id id ]
        [ div [ class "panel-heading" ]
            [ div [ class "row" ]
                [ strong [ class "col-sm-6" ]
                    [ a [ href <| "./#" ++ id ] [ text <| "Firefox " ++ details.version ] ]
                , em [ class "col-sm-6 text-right" ] [ text <| formatDate last_modified ]
                ]
            ]
        , div [ class "panel-body" ]
            [ viewReleaseDetails details
            , h4 [] [ text "System Addons" ]
            , viewSystemAddons builtins updates
            ]
        ]


filterCheckbox : (String -> Bool -> Msg) -> ( String, Bool ) -> Html Msg
filterCheckbox handler ( name, active ) =
    li [ class "list-group-item" ]
        [ div [ class "checkbox", style [ ( "margin", "0" ) ] ]
            [ label []
                [ input
                    [ type_ "checkbox", onCheck <| handler name, checked active ]
                    []
                , text <| " " ++ name
                ]
            ]
        ]


filterSetForm : List ( String, Bool ) -> String -> (String -> Bool -> Msg) -> Html Msg
filterSetForm filterSet label handler =
    let
        filters =
            filterSet |> List.map (filterCheckbox handler)
    in
        div [ class "panel panel-default" ]
            [ div [ class "panel-heading" ] [ strong [] [ text label ] ]
            , ul [ class "list-group" ] <| filters
            ]


viewFilters : Model -> Html Msg
viewFilters { filters } =
    let
        channels =
            Dict.toList filters.channels

        versions =
            Dict.toList filters.versions |> List.reverse

        targets =
            Dict.toList filters.targets

        addons =
            Dict.toList filters.addons

        eventHandler msg =
            (\name active -> ToggleFilter <| msg name active)
    in
        div
            [ style
                [ ( "position", "fixed" )
                , ( "max-height", "calc(100vh - 75px)" )
                , ( "position", "fixed" )
                , ( "overflow-y", "auto" )
                , ( "padding-right", ".1em" )
                ]
            ]
            [ filterSetForm channels "Channels" <| eventHandler ToggleChannel
            , filterSetForm versions "Versions" <| eventHandler ToggleVersion
            , filterSetForm targets "Targets" <| eventHandler ToggleTarget
            , filterSetForm addons "Addons" <| eventHandler ToggleAddon
            ]


spinner : Html Msg
spinner =
    div [ class "loader-wrapper" ] [ div [ class "loader" ] [] ]


view : Model -> Html Msg
view ({ loading } as model) =
    div [ class "container" ]
        [ div [ class "header" ]
            [ h1 [] [ Html.text "System Addons" ] ]
        , div [ class "row" ] <|
            if loading then
                [ spinner ]
            else
                [ div [ class "col-sm-9" ]
                    [ div [] <| List.map viewRelease <| applyFilters model ]
                , div [ class "col-sm-3" ]
                    [ viewFilters model ]
                ]
        ]
