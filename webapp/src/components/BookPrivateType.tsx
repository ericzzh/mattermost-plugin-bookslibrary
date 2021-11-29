//System
import React from "react";
import { useHistory } from "react-router-dom";
import styled from "@emotion/styled";
import { useSelector } from "react-redux";

//MaterialUI
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import TextField from "@material-ui/core/TextField";
import HouseIcon from "@material-ui/icons/House";
import Chip from "@material-ui/core/Chip";
import IconButton from "@material-ui/core/IconButton";
import Tabs from "@material-ui/core/Tabs";
import Tab from "@material-ui/core/Tab";
import {
    DataGrid,
    GridColDef,
} from "@material-ui/data-grid";

//MaterialIcon
import BookIcon from "@material-ui/icons/Book";

//MaterialUI Sytle
import { useTheme, StylesProvider } from "@material-ui/core/styles";

//mattermost redux
import { getCurrentTeam } from "mattermost-redux/selectors/entities/teams";

// const { formatText, messageHtmlToComponent } = window.PostUtils;

const TEXT: Record<string, string> = {
    LABEL_BOOKID: "ID",
    LABEL_BOOKNAME: "图书名",
    TAB_GENERL: "一般信息",
    TAB_COPY: "书册信息",
    HEADER_KEEPER: "保管员",
    HEADER_COPYID: "书册编号",
};

function BookPrivateType(props: any) {
    const post = { ...props.post };
    const message = post.message || "";
    const history = useHistory();
    const defaultTheme = useTheme();

    //Current Team
    const currentTeam = useSelector(getCurrentTeam);

    //States
    const [value, setValue] = React.useState(0);

    let bookPrivate: BookPrivate;
    try {
        bookPrivate = JSON.parse(message);
    } catch (error) {
        // const formattedText = messageHtmlToComponent(formatText(message));
        // return <div> {formattedText} </div>;
        return <div>{message}</div>;
    }

    type nameByUser = {
        [key: string]: string;
    };

    const keeperUsers = bookPrivate.keeper_users;
    const nameByUser: { [key: string]: string } = {};
    for (let index = 0; index < keeperUsers.length; index++) {
        const user = keeperUsers[index];
        nameByUser[user] = bookPrivate.keeper_infos && bookPrivate.keeper_infos[user].name;
    }

    const StyledBookPrivate = styled(Paper)(() => {
        const theme = defaultTheme;
        return {
            padding: theme.spacing(2),
            margin: "auto",
            maxWidth: "100%",
            position: "relative",
            "& label,input": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "1rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "1.5rem",
                },
            },
            "& .PaticipantCommon": {
                [theme.breakpoints.up("xs")]: {
                    fontSize: "0.8rem",
                },
                [theme.breakpoints.up("sm")]: {
                    fontSize: "0.8rem",
                },
            },

            "& .PaticipantKeeper": {
                backgroundColor: "teal",
            },
        };
    });

    const handleRoleUserClick = (userName: string) => {
        return () => history.push(`/${currentTeam.name}/messages/@${userName}`);
    };
    const handleLinkToBook = () =>
        history.push(
            `/${currentTeam.name}/pl/${bookPrivate.relations_pri.public}`
        );

    const generalInfo = (
        <Grid container spacing={2} alignItems={"flex-end"}>
            <Grid item>
                <TextField
                    label={TEXT["LABEL_BOOKID"]}
                    value={bookPrivate.id_pri}
                    InputProps={{
                        readOnly: true,
                    }}
                />
            </Grid>
            <Grid item>
                <TextField
                    label={TEXT["LABEL_BOOKNAME"]}
                    value={bookPrivate.name_pri}
                    InputProps={{
                        readOnly: true,
                    }}
                />
                <IconButton onClick={handleLinkToBook}>
                    <BookIcon />
                </IconButton>
            </Grid>
            {bookPrivate.keeper_infos && Object.keys(bookPrivate.keeper_infos).map((keeperUser) => (
                <Grid item>
                    <Chip
                        size={"medium"}
                        icon={<HouseIcon />}
                        color="primary"
                        label={bookPrivate.keeper_infos[keeperUser].name}
                        className={"PaticipantCommon PaticipantKeeper"}
                        clickable
                        onClick={handleRoleUserClick(keeperUser)}
                    />
                </Grid>
            ))}
        </Grid>
    );

    const rows: { id: number; name: string; copyid: string }[] = [];

    const copies = bookPrivate.copy_keeper_map;
    if (copies) {
        let id = 1;
        for (const copyid in copies) {
            rows.push({
                id: id++,
                name: nameByUser[copies[copyid].user],
                copyid: copyid,
            });
        }
    }

    const columns: GridColDef[] = [
        {
            field: "name",
            headerName: TEXT["HEADER_KEEPER"],
            resizable:true,
            flex:35,
        },
        {
            field: "copyid",
            headerName: TEXT["HEADER_COPYID"],
            resizable:true,
            flex:65,
        },
    ];

    const StyledDataGridContainer = styled("div")(() => {
        const theme = defaultTheme;
        return {
            height: 200,
            width: "100%",
            ".MuiDataGrid-root": {
                fontSize: "1rem",
            },
        };
    });
    const displayTab = (index: number) => {
        switch (index) {
            case 0:
                return generalInfo;
            case 1:
                return (
                    <StyledDataGridContainer>
                        <DataGrid
                            rows={rows}
                            columns={columns}
                            density={"compact"}
                            hideFooterPagination={true}
                            disableSelectionOnClick={true}
                            disableColumnMenu={true}
                        />
                        ;
                    </StyledDataGridContainer>
                );
            default:
                break;
        }
    };

    const StyledTab = styled(Tabs)(() => {
        const theme = defaultTheme;
        return {
            ".MuiTab-root": {
                fontSize: "1.2rem",
            },
        };
    });

    const handleChange = (event: React.ChangeEvent<{}>, newValue: number) => {
        setValue(newValue);
    };

    return (
        <StylesProvider injectFirst>
            <StyledBookPrivate>
                <StyledTab value={value} onChange={handleChange}>
                    <Tab label={TEXT["TAB_GENERL"]} id={"pri-tab-01"} />
                    <Tab label={TEXT["TAB_COPY"]} id={"pri-tab-02"} />
                </StyledTab>
                {displayTab(value)}
            </StyledBookPrivate>
        </StylesProvider>
    );
}

export default React.memo(
    BookPrivateType,
    (prev, next) => prev.post.message === next.post.message
);
