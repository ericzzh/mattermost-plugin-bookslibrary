import React from "react";
import { useHistory } from "react-router-dom";
import styled from "@emotion/styled";
import { useSelector } from "react-redux";

import { useTheme, StylesProvider } from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import TextField from "@material-ui/core/TextField";

import IconButton from "@material-ui/core/IconButton";

import Tabs from "@material-ui/core/Tabs";
import Tab from "@material-ui/core/Tab";
import { DataGrid, GridColDef } from "@material-ui/data-grid";

//MaterialIcon
import BookIcon from "@material-ui/icons/Book";

//mattermost redux
import { getCurrentTeam } from "mattermost-redux/selectors/entities/teams";
// const { formatText, messageHtmlToComponent } = window.PostUtils;

const TEXT: Record<string, string> = {
    LABEL_BOOKID: "ID",
    LABEL_BOOKNAME: "图书名",
    LABEL_STOCK: "在库",
    LABEL_TRANSMIT_OUT: "正在转出",
    LABEL_LENDING: "正在借阅",
    LABEL_TRANSMIT_IN: "正在转入",
    LABEL_PUBLIC_LINK: "图书链接",
    TAB_GENERL: "一般信息",
    TAB_COPY: "书册信息",
    HEADER_COPYID: "书册编号",
    HEADER_STATUS: "状态",
};

function BookInventoryType(props: any) {
    const post = { ...props.post };
    const message = post.message || "";
    const history = useHistory();
    const defaultTheme = useTheme();

    //States
    const [value, setValue] = React.useState(0);

    //Current Team
    const currentTeam = useSelector(getCurrentTeam);
    let bookInventory: BookInventory;
    try {
        bookInventory = JSON.parse(message);
    } catch (error) {
        // const formattedText = messageHtmlToComponent(formatText(message));
        // return <div> {formattedText} </div>;
        return <div> {message} </div>;
    }

    const handleLinkToBook = () =>
        history.push(
            `/${currentTeam.name}/pl/${bookInventory.relations_inv.public}`
        );

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
        };
    });

    const generalInfo = (
        <Grid container spacing={2}>
            <Grid item>
                <TextField
                    label={TEXT["LABEL_BOOKID"]}
                    value={bookInventory.id_inv}
                    InputProps={{
                        readOnly: true,
                    }}
                />
            </Grid>
            <Grid item>
                <TextField
                    label={TEXT["LABEL_BOOKNAME"]}
                    value={bookInventory.name_inv}
                    InputProps={{
                        readOnly: true,
                    }}
                />
            </Grid>
            <Grid item>
                <TextField
                    label={TEXT["LABEL_STOCK"]}
                    value={bookInventory.stock}
                    InputProps={{
                        readOnly: true,
                    }}
                />
            </Grid>
            <Grid item>
                <TextField
                    label={TEXT["LABEL_TRANSMIT_OUT"]}
                    value={bookInventory.transmit_out}
                    InputProps={{
                        readOnly: true,
                    }}
                />
            </Grid>
            <Grid item>
                <TextField
                    label={TEXT["LABEL_LENDING"]}
                    value={bookInventory.lending}
                    InputProps={{
                        readOnly: true,
                    }}
                />
            </Grid>
            <Grid item>
                <TextField
                    label={TEXT["LABEL_TRANSMIT_IN"]}
                    value={bookInventory.transmit_in}
                    InputProps={{
                        readOnly: true,
                    }}
                />
            </Grid>
            <Grid item>
                <IconButton onClick={handleLinkToBook}>
                    <BookIcon />
                </IconButton>
            </Grid>
        </Grid>
    );

    const rows: { id: number; copyid: string; status: string }[] = [];

    if (!bookInventory.copies) {
        bookInventory.copies = {};
    }

    const copyids = Object.keys(bookInventory.copies);
    const sortedCopyids = copyids.sort();
    let id = 0;
    const getStatusLabel = (status: string) => {
        switch (status) {
            case "in_stock":
                return TEXT["LABEL_STOCK"];
            case "transmit_in":
                return TEXT["LABEL_TRANSMIT_IN"];
            case "transmit_out":
                return TEXT["LABEL_TRANSMIT_OUT"];
            case "lending":
                return TEXT["LABEL_LENDING"];
            default:
                break;
        }
        return "";
    };
    for (const copyid of sortedCopyids) {
        rows.push({
            id: id++,
            copyid: copyid,
            status: getStatusLabel(bookInventory.copies[copyid].status),
        });
    }

    const columns: GridColDef[] = [
        {
            field: "copyid",
            headerName: TEXT["HEADER_COPYID"],
            flex:65,
        },
        {
            field: "status",
            headerName: TEXT["HEADER_STATUS"],
            flex:35,
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
                    <Tab label={TEXT["TAB_GENERL"]} id={"inv-tab-01"} />
                    <Tab label={TEXT["TAB_COPY"]} id={"inv-tab-02"} />
                </StyledTab>
                {displayTab(value)}
            </StyledBookPrivate>
        </StylesProvider>
    );
}

export default React.memo(
    BookInventoryType,
    (prev, next) => prev.post.message === next.post.message
);
