import React from "react";
import PropTypes from "prop-types";
import { makeStyles } from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import Typography from "@material-ui/core/Typography";
import Breadcrumbs from "@material-ui/core/Breadcrumbs";
import Link from "@material-ui/core/Link";
import Fab from "@material-ui/core/Fab";
import BorrowIcon from "@material-ui/icons/MenuBook";
import WorkerIcon from "@material-ui/icons/PermContactCalendar";
import { useDispatch } from "react-redux";
import { searchForTerm } from "actions/post_action";
import { Client4 } from "mattermost-redux/client";
import manifest from "../manifest";
import { getCurrentUser } from "mattermost-redux/selectors/entities/common";
import { getConfig } from "mattermost-redux/selectors/entities/general";
import { useSelector } from "react-redux";
import Accordion from "@material-ui/core/Accordion";
import AccordionSummary from "@material-ui/core/AccordionSummary";
import AccordionDetails from "@material-ui/core/AccordionDetails";
import ExpandMoreIcon from "@material-ui/icons/ExpandMore";
import Switch from "@material-ui/core/Switch";
import Chip from "@material-ui/core/Chip";
import Button from "@material-ui/core/Button";
import ButtonGroup from "@material-ui/core/ButtonGroup";
import useMediaQuery from "@material-ui/core/useMediaQuery";
import { useTheme } from "@material-ui/core/styles";

interface Book {
    post_id: string;
    id: string;
    name: string;
    name_en: string;
    category1: string;
    category2: string;
    category3: string;
    author: string;
    author_en: string;
    translator: string;
    translator_en: string;
    publisher: string;
    publisher_en: string;
    publish_date: string;
    introduction: string;
    book_index: string;
    libworker_users: string[];
    libworker_names: string[];
    keeper_users: string[];
    keeper_names: string[];
    isAllowedToBorrow: boolean;
    tags: string[];
}

interface BorrowRequestKey{
    book_post_id: string;
    borrower_user: string;
  }

const { formatText, messageHtmlToComponent } = window.PostUtils;

BookType.propTypes = {
    post: PropTypes.object.isRequired,
    theme: PropTypes.object.isRequired,
};

const useStyles = makeStyles((theme) => ({
    bookc_paper: {
        padding: theme.spacing(2),
        margin: "auto",
        maxWidth: "100%",
    },
    bookc_img: {
        maxWidth: "100%",
        maxHeight: "100%",
        float: "left",
    },
    img_wrapper: {
        [theme.breakpoints.up("xs")]: {
            width: 125 * 0.8,
            height: 160 * 0.8,
        },
        [theme.breakpoints.up("sm")]: {
            width: 125 * 1.5,
            height: 160 * 1.5,
        },
    },
    bookc_button: {
        width: 50,
        height: 50,
    },
    longInfoWithAcc: {
        width: "100%",
    },
    bookCategoryOl: {
        paddingLeft: "0 !important",
    },
    bookCategory: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "0.8rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1.2rem",
        },
    },
    bookName: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "1.5rem",
            fontWeight: "bold",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "3rem",
            fontWeight: "bold",
        },
    },
    englishName: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "0.8rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1rem",
        },
    },
    authorName: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "1rem",
            fontWeight: "bold",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1.5rem",
            fontWeight: "bold",
        },
    },
    bookAttr: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "0.8rem",
            marginTop: "-1.5rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1rem",
        },
    },
    intro: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "1rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1.5rem",
        },
    },
    index: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "1rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1.5rem",
        },
    },
    libworker: {
        [theme.breakpoints.up("xs")]: {
            fontSize: "0.8rem",
        },
        [theme.breakpoints.up("sm")]: {
            fontSize: "1rem",
        },
    },

    libworkerColor: {
        backgroundColor: "green",
    },
}));

function BookType(props: any) {
    const classes = useStyles();
    const post = { ...props.post };
    const message = post.message || "";
    const dispatch = useDispatch();
    const currentUser = useSelector(getCurrentUser);
    const config = useSelector(getConfig);
    const theme = useTheme();
    const matchesSm = useMediaQuery(theme.breakpoints.up("sm"));
    const matchesXs = useMediaQuery(theme.breakpoints.up("xs"));
    let book: Book;

    try {
        book = JSON.parse(message);
    } catch (error) {
        const formattedText = messageHtmlToComponent(formatText(message));
        return <div> {formattedText} </div>;
    }

    // It's enough to just send back a post id.
    const handleBorrow = async () => {
        const request: BorrowRequestKey ={
              book_post_id: props.post.id,
              borrower_user: currentUser.username,
          }
        const data = await Client4.doFetch<{ error: string }>(
            `/plugins/${manifest.id}/borrow`,
            {
                method: "POST",
                body: JSON.stringify(request),
            }
        );

        if (!data.error) {
            console.error(data.error);
        }
    };

    // <Link
    //     color="inherit"
    //     onClick={(e) =>
    //         dispatch(searchForTerm("#" + book.publisher))
    //     }
    // >
    //     {book.publisher}
    // </Link>
    const bookName = (
        <>
            <div className={classes.bookName}>{book.name}</div>
            <div className={classes.englishName}>{book.name_en}</div>
            {matchesSm ? <br /> : <div />}
        </>
    );

    const author = (
        <>
            <div className={classes.authorName}>{book.author}</div>
            <div className={classes.englishName}>{book.author_en}</div>
            <br />
        </>
    );
    const publisher = (
        <>
            <div className={classes.bookAttr}>{book.publisher}</div>
            <div className={classes.englishName}>{book.publisher_en}</div>
        </>
    );
    const pulishDate = (
        <>
            <div className={classes.bookAttr}>{book.publish_date}</div>
        </>
    );
    const translator = (
        <>
            <div className={classes.bookAttr}>{book.translator}</div>
        </>
    );

    const bookAttribute = (
        <Grid container direction={"column"} spacing={2}>
            <Grid item>{author}</Grid>
            <Grid item container spacing={2}>
                <Grid item>{translator}</Grid>
                <Grid item>{publisher}</Grid>
                <Grid item>{pulishDate}</Grid>
            </Grid>
        </Grid>
    );

    const bookIndex = (
        <Grid container>
            <Accordion className={classes.longInfoWithAcc}>
                <AccordionSummary
                    expandIcon={<ExpandMoreIcon />}
                    id="bookIndex-header"
                >
                    <div>{"目录"}</div>
                </AccordionSummary>
                <AccordionDetails id="bookIndex-detail">
                    {book.book_index ? book.book_index : "暂无"}
                </AccordionDetails>
            </Accordion>
        </Grid>
    );

    const bookIntro = (
        <Grid container>
            <Accordion className={classes.longInfoWithAcc}>
                <AccordionSummary
                    expandIcon={<ExpandMoreIcon />}
                    id="introduction-header"
                >
                    <div>{"简介"}</div>
                </AccordionSummary>
                <AccordionDetails id="introduction-detail">
                    {book.introduction ? book.introduction : "暂无"}
                </AccordionDetails>
            </Accordion>
        </Grid>
    );

    const bookInfo = (
        <Grid container spacing={2}>
            <Grid item xs={12}>
                <Typography>
                    <div className={classes.intro}>{bookIntro}</div>
                </Typography>
            </Grid>
            <Grid item xs={12}>
                <Typography>
                    <div className={classes.index}>{bookIndex}</div>
                </Typography>
            </Grid>
        </Grid>
    );

    const bookState = (
        <Grid
            container
            alignItems={"center"}
            spacing={2}
            justifyContent={"flex-end"}
        >
            {book.libworker_names &&
                book.libworker_names.map((worker) => (
                    <>
                        {
                            <Grid item className={classes.authorName}>
                                <Chip
                                    classes={{
                                        label: classes.libworker,
                                        colorPrimary: classes.libworkerColor,
                                    }}
                                    color="primary"
                                    size="medium"
                                    label={worker}
                                    icon={<WorkerIcon />}
                                />
                            </Grid>
                        }
                    </>
                ))}
            <Grid item>
                <Fab
                    color="primary"
                    aria-label="borrow"
                    className={classes.bookc_button}
                    onClick={handleBorrow}
                >
                    <BorrowIcon />
                </Fab>
            </Grid>
        </Grid>
    );

    const bookBreadcrumb = (
        <>
            <Breadcrumbs
                classes={{
                    li: classes.bookCategory,
                    ol: classes.bookCategoryOl,
                }}
            >
                <Link color="inherit">{book.category1}</Link>
                <Link color="inherit">{book.category2}</Link>
                <Link color="inherit">{book.category3}</Link>
            </Breadcrumbs>
        </>
    );

    const bookMain = (
        <>
            <Grid container spacing={1}>
                <Grid
                    item
                    xs={matchesSm ? 2 : 4}
                    className={classes.img_wrapper}
                >
                    <img
                        className={classes.bookc_img}
                        alt="no image"
                        src={`${config.SiteURL}/plugins/${manifest.id}/public/s4216567.jpeg`}
                    />
                </Grid>
                <Grid item xs={matchesSm ? 10 : 8}>
                    <Grid container direction={"column"}>
                        <Grid item> {bookBreadcrumb} </Grid>
                        <Grid item>{bookName}</Grid>
                        <Grid item> {bookAttribute} </Grid>
                    </Grid>
                </Grid>
            </Grid>
        </>
    );

    const main = (
        <Grid container direction="column" spacing={2}>
            <Grid item>{bookMain}</Grid>
            <Grid item>{bookInfo}</Grid>
            <Grid item>{bookState}</Grid>
        </Grid>
    );

    const buttons = (
        <Grid container justifyContent={"flex-end"} alignItems={"center"}>
            <Grid item>
                <ButtonGroup size="small">
                    <Button>图片</Button>
                    <Button>简介</Button>
                    <Button>目录</Button>
                </ButtonGroup>
            </Grid>
            <Grid item>
                <Switch name="isAllowedToBorrow" />
            </Grid>
        </Grid>
    );

    return (
        <Paper className={classes.bookc_paper}>
            <Grid container direction={"column"}>
                <Grid item>{main}</Grid>
                <Grid item>{buttons}</Grid>
            </Grid>
        </Paper>
    );
}

export default React.memo(BookType);
